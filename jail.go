/*
(BSD 2-clause license)

Copyright (c) 2014, Shawn Webb
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

   * Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package jail

/*
 * The jail.Jail object implements the VirtualMachine interface
 */

import (
    /* "fmt" */
    "os/exec"
    "github.com/nu7hatch/gouuid"
    "github.com/coopernurse/gorp"
    "github.com/virtbsd/network"
    "github.com/virtbsd/VirtualMachine"
    "github.com/virtbsd/zfs"
)

type MountPoint struct {
    JailUUID string
    Source string
    Destination string
    Options string
    Driver string
    MountOrder int
}

type JailOption struct {
    JailUUID string
    OptionKey string
    OptionValue string
}

type Jail struct {
    UUID string
    Name string
    HostName string
    CreateDate int
    ModificationDate int
    ZFSDataset string

    NetworkDevices []*network.NetworkDevice `db:"-"`
    Mounts []*MountPoint `db:"-"`
    Options []*JailOption `db:"-"`
    BootEnvironments map[string]bool `db:"-"`
    Snapshots []string `db:"-"`
    ZFSDatasetObj *zfs.Dataset `db:"-"`
    Routes []*network.Route `db:"-"`

    Path string `db:"-"`
    Dirty bool `db:"-"`
}

func (jail *Jail) PostGet(s gorp.SqlExecutor) error {
    jail.NetworkDevices = network.GetNetworkDevices(map[string]interface{}{"sqlexecutor": s}, jail)

    s.Select(&jail.Mounts, "select * from MountPoint where JailUUID = ? order by MountOrder", jail.UUID)
    s.Select(&jail.Options, "select * from JailOption where JailUUID = ?", jail.UUID)
    s.Select(&jail.Routes, "select * from Route WHERE VmUUID = ?", jail.UUID)
    if len(jail.HostName) == 0 {
        jail.HostName = jail.Name
    }

    jail.ZFSDatasetObj = zfs.GetDataset(jail.ZFSDataset)

    return nil
}

func (jail *Jail) GetUUID() string {
    return jail.UUID
}

func LookupUUID(db *gorp.DbMap, field map[string]interface{}) string {
    fields := []string{ "name", "hostname" }

    if uuid, ok := field["uuid"]; ok == true {
        return uuid.(string)
    }

    for i := 0; i < len(fields); i++ {
        if val, ok := field[fields[i]]; ok == true {
            myuuid, err := db.SelectStr("select UUID from jail where " + fields[i] + " = ?", val)
            if err == nil {
                return myuuid
            }
        }
    }

    return ""
}

func GetJail(db *gorp.DbMap, field map[string]interface{}) *Jail {
    uuid := LookupUUID(db, field)
    if len(uuid) == 0 {
        return nil
    }

    obj, err := db.Get(Jail{}, uuid)
    if err != nil {
        panic(err)
        return nil
    }

    if obj == nil {
        /* Jail not found */
        return nil
    }

    return obj.(*Jail)
}

func (jail *Jail) Start() error {
    path := jail.GetPath()

    if jail.IsOnline() == true {
        return nil
    }

    cmd := exec.Command("/sbin/mount", "-t", "devfs", "devfs", path + "/dev")
    if err := cmd.Run(); err != nil {
        return err
    }

    cmd = exec.Command("/usr/sbin/jail", "-c", "vnet", "name=" + jail.UUID, "host.hostname=" + jail.HostName, "path=" + path, "persist")
    if err := cmd.Run(); err != nil {
        return err
    }

    for i := range jail.Mounts {
        cmd = exec.Command("/usr/sbin/jexec", jail.UUID, "/sbin/mount")
        if len(jail.Mounts[i].Driver) > 0 {
            cmd.Args = append(cmd.Args, "-t")
            cmd.Args = append(cmd.Args, jail.Mounts[i].Driver)
        }

        if len(jail.Mounts[i].Options) > 0 {
            cmd.Args = append(cmd.Args, "-o")
            cmd.Args = append(cmd.Args, jail.Mounts[i].Options)
        }

        cmd.Args = append(cmd.Args, jail.Mounts[i].Source)
        cmd.Args = append(cmd.Args, path + "/" + jail.Mounts[i].Destination)

        if err := cmd.Run(); err != nil {
            return err
        }
    }

    return nil
}

func (jail *Jail) Stop() error {
    return nil
}

func (jail *Jail) Status() string {
    if jail.IsOnline() {
        return "Online"
    } else {
        return "Offline"
    }
}

func (jail *Jail) CreateSnapshot(snapname string) error {
    return nil
}

func (jail *Jail) RestoreSnapshot(snapname string) error {
    return nil
}

func (jail *Jail) DeleteSnapshot(snapname string) error {
    return nil
}

func (jail *Jail) PrepareHostNetworking() error {
    for i := range jail.NetworkDevices {
        if err := jail.NetworkDevices[i].BringHostOnline(); err != nil {
            return err
        }
    }

    return nil
}

func (jail *Jail) PrepareGuestNetworking() error {
    for i := range jail.NetworkDevices {
        if err := jail.NetworkDevices[i].BringGuestOnline(jail); err != nil {
            return err
        }
    }

    cmd := exec.Command("/usr/sbin/jexec", jail.UUID, "/sbin/ifconfig", "lo0", "inet", "127.0.0.1", "up")
    if err := cmd.Run(); err != nil {
        return err
    }

    return nil
}

func (jail *Jail) NetworkingStatus() string {
    return ""
}

func (jail *Jail) GetPath() string {
    if len(jail.Path) > 0 {
        return jail.Path
    }

    path, err := zfs.GetDatasetPath(jail.ZFSDataset)
    if err != nil {
        panic(err)
        return ""
    }

    jail.Path = path

    return path
}

func (jail *Jail) IsOnline() bool {
    cmd := exec.Command("/usr/sbin/jls", "-j", jail.UUID)
    err := cmd.Run()
    if err == nil {
        return true
    }

    return false
}

func (jail *Jail) Validate() error {
    if len(jail.UUID) == 0 {
        /* If we haven't been persisted (this is a new jail), then we don't have a UUID */
        myuuid, _ := uuid.NewV4()
        jail.UUID = myuuid.String()
    }

    if _, err := uuid.ParseHex(jail.UUID); err != nil {
        return VirtualMachine.VirtualMachineError{"Invalid UUID", jail}
    }

    if len(jail.GetPath()) == 0 {
        return VirtualMachine.VirtualMachineError{"Invalid Path, ZFS Dataset: " + jail.ZFSDataset, jail}
    }

    return nil
}

func (jail *Jail) Persist(db *gorp.DbMap) error {
    if err := jail.Validate(); err != nil {
        return err
    }

    return nil
}

func (jail *Jail) Delete(db *gorp.DbMap) error {
    return nil
}

func (jail *Jail) Archive(archivename string) error {
    return nil
}
