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
    "strings"
    "strconv"
    "fmt"
    "os/exec"
    "encoding/json"
    "github.com/nu7hatch/gouuid"
    "github.com/coopernurse/gorp"
    "github.com/virtbsd/network"
    "github.com/virtbsd/VirtualMachine"
    "github.com/virtbsd/zfs"
    "github.com/virtbsd/util"
)

type MountPoint struct {
    MountPointID int
    JailUUID string
    Source string
    Destination string
    Options string
    Driver string
    MountOrder int
}

type JailOption struct {
    OptionID int
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
    ZFSDatasetObj *zfs.Dataset `db:"-" json:"-"`
    Routes []*network.Route `db:"-"`

    Path string `db:"-"`
    Dirty bool `db:"-"`
}

type JailJSON struct {
    UUID string
    Name string
    HostName string
    ZFSDataset string
    Path string
    Status string

    NetworkDevices []*network.NetworkDevice
    BootEnvironments map[string]bool
}

func (jail *Jail) PostGet(s gorp.SqlExecutor) error {
    jail.NetworkDevices = network.GetNetworkDevices(map[string]interface{}{"sqlexecutor": s}, jail)

    s.Select(&jail.Mounts, "select * from MountPoint where JailUUID = ? order by MountOrder", jail.UUID)
    s.Select(&jail.Options, "select * from JailOption where JailUUID = ?", jail.UUID)
    s.Select(&jail.Routes, "select * from Route WHERE VmUUID = ?", jail.UUID)
    if len(jail.HostName) == 0 {
        jail.HostName = jail.Name
    }

    jail.BootEnvironments = make(map[string]bool)

    jail.ZFSDatasetObj = zfs.GetDataset(jail.ZFSDataset)
    for _, rootDataset := range jail.ZFSDatasetObj.Children {
        if strings.HasPrefix(rootDataset.DatasetPath, jail.ZFSDataset + "/ROOT") {
            for _, dataset := range rootDataset.Children {
                if _, ok := dataset.Options["jailadmin:be_active"]; ok == true {
                    jail.BootEnvironments[dataset.DatasetPath], _ = strconv.ParseBool(dataset.Options["jailadmin:be_active"])
                }
            }

            break
        }
    }

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

func GetAllJails(db *gorp.DbMap) []*Jail {
    var uuids []string
    var jails []*Jail

    if _, err := db.Select(&uuids, "select UUID from Jail"); err != nil {
        return nil
    }

    for _, uuid := range uuids {
        jails = append(jails, GetJail(db, map[string]interface{} {"uuid": uuid}))
    }

    return jails
}

func (jail *Jail) Start() error {
    path, err := jail.GetPath()
    if err != nil {
        return err
    }

    if jail.IsOnline() == true {
        return nil
    }

    cmd := exec.Command("/sbin/mount", "-t", "devfs", "devfs", path + "/dev")
    if rawoutput, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("Error mount devfs in jail: %s", virtbsdutil.ByteToString(rawoutput))
        return err
    }

    cmd = exec.Command("/usr/sbin/jail", "-c", "vnet", "name=" + jail.UUID, "host.hostname=" + jail.HostName, "path=" + path, "persist")
    for i := range jail.Options {
        opt := jail.Options[i].OptionKey
        if len(jail.Options[i].OptionValue) > 0 {
            opt += jail.Options[i].OptionValue
        }

        cmd.Args = append(cmd.Args, opt)
    }

    if rawoutput, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("Error starting jail: %s", virtbsdutil.ByteToString(rawoutput))
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

        if rawoutput, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("Error mounting %s: %s", jail.Mounts[i].Destination, virtbsdutil.ByteToString(rawoutput))
        }
    }

    cmd = exec.Command("/usr/sbin/jexec", jail.UUID, "/bin/sh", "/etc/rc")
    if rawoutput, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("Error running /etc/rc: %s", virtbsdutil.ByteToString(rawoutput))
    }

    return nil
}

func (jail *Jail) Stop() error {
    path, err := jail.GetPath()
    if err != nil {
        return err
    }

    if jail.IsOnline() == false {
        return nil
    }

    cmd := exec.Command("/usr/sbin/jail", "-r", jail.UUID)
    if err := cmd.Run(); err != nil {
        return nil
    }

    for i := range jail.Mounts {
        cmd = exec.Command("/sbin/umount", path + "/" + jail.Mounts[i].Destination)
        if rawoutput, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("/sbin/unmount %s/%s: %s", path, jail.Mounts[i].Destination, virtbsdutil.ByteToString(rawoutput))
        }
    }

    for i := range jail.NetworkDevices {
        if err := jail.NetworkDevices[i].BringOffline(); err != nil {
            return err
        }
    }

    cmd = exec.Command("/sbin/umount", path + "/dev")
    if rawoutput, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("/sbin/unmount %s/dev: %s\n", path, virtbsdutil.ByteToString(rawoutput))
    }

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

    for _, route := range jail.Routes {
        proto := "-inet"
        if strings.Index(route.Source, ":") >= 0 {
            proto = "-inet6"
        }

        cmd = exec.Command("/usr/bin/jexec", jail.UUID, "/sbin/route", "add", proto, route.Source, route.Destination)
        if rawoutput, err := cmd.CombinedOutput(); err != nil {
            return fmt.Errorf("Adding route for [%s] to [%s] failed: %s", route.Source, route.Destination, virtbsdutil.ByteToString(rawoutput))
        }
    }

    return nil
}

func (jail *Jail) PostStart() error {
    /* FixUp for IPv6 - FreeBSD's DAD can sometimes go haywire */
    for _, device := range jail.NetworkDevices {
        has_ipv6 := false
        for _, address := range device.Addresses {
            if strings.Index(address, ":")  >= 0 {
                has_ipv6 = true
            }
        }

        if has_ipv6 {
            cmd := exec.Command("/usr/bin/jexec", jail.UUID, "/sbin/ifconfig", "epair" + strconv.Itoa(device.DeviceID) + "b", "inet6", "-ifdisabled")
            if rawoutput, err := cmd.CombinedOutput(); err != nil {
                return fmt.Errorf("Could not enable IPv6 for epair%db: %s", device.DeviceID, virtbsdutil.ByteToString(rawoutput))
            }
        }
    }

    return nil
}

func (jail *Jail) NetworkingStatus() string {
    return ""
}

func (jail *Jail) GetPath() (string, error) {
    if len(jail.Path) > 0 {
        return jail.Path, nil
    }

    if len(jail.BootEnvironments) > 0 {
        for k, v := range jail.BootEnvironments {
            if v == true {
                path, err := zfs.GetDatasetPath(k)
                if err == nil && len(path) > 0 {
                    jail.Path = path
                    return path, nil
                }
            }
        }

        return "", fmt.Errorf("Boot environments enabled. No active boot environment found.")
    }

    path, err := zfs.GetDatasetPath(jail.ZFSDataset)
    if err != nil {
        return "", err
    }

    jail.Path = path

    return path, nil
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

    if path, err := jail.GetPath(); err != nil || len(path) == 0 {
        return VirtualMachine.VirtualMachineError{"Invalid Path, ZFS Dataset: " + jail.ZFSDataset, jail}
    }

    return nil
}

func (jail *Jail) Persist(db *gorp.DbMap) error {
    if err := jail.Validate(); err != nil {
        return err
    }

    for _, device := range jail.NetworkDevices {
        if err := device.Persist(db, jail); err != nil {
            return err
        }
    }

    for _, mount := range jail.Mounts {
        if mount.MountPointID == 0 {
            db.Insert(mount)
        } else {
            db.Update(mount)
        }
    }

    for _, option := range jail.Options {
        if option.OptionID == 0 {
            db.Insert(option)
        } else {
            db.Update(option)
        }
    }

    return nil
}

func (jail *Jail) Delete(db *gorp.DbMap) error {
    return nil
}

func (jail *Jail) Archive(archivename string) error {
    return nil
}

func (jail *Jail) MarshalJSON() ([]byte, error) {
    obj := JailJSON{}
    obj.UUID = jail.UUID
    obj.Name = jail.Name
    obj.HostName = jail.HostName
    obj.ZFSDataset = jail.ZFSDataset
    obj.Path, _ = jail.GetPath()
    obj.Status = jail.Status()
    obj.NetworkDevices = jail.NetworkDevices
    obj.BootEnvironments = jail.BootEnvironments

    bytes, err := json.MarshalIndent(obj, "", "    ")
    return bytes, err
}
