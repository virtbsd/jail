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
    "github.com/coopernurse/gorp"
    "github.com/virtbsd/network"
)

type MountPoint struct {
    JailUUID string
    Source string
    Destination string
    Options string
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

    Dirty bool `db:"-"`
}

func (jail *Jail) PostGet(s gorp.SqlExecutor) error {
    jail.NetworkDevices = network.GetNetworkDevices(map[string]interface{}{"sqlexecutor": s}, *jail)

    s.Select(&jail.Mounts, "select * from MountPoint where JailUUID = ?", jail.UUID)
    s.Select(&jail.Options, "select * from JailOption where JailUUID = ?", jail.UUID)

    return nil
}

func (jail Jail) GetUUID() string {
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

func (jail Jail) Start() error {
    return nil
}

func (jail Jail) Stop() error {
    return nil
}

func (jail Jail) Status() string {
    return ""
}

func (jail Jail) CreateSnapshot(snapname string) error {
    return nil
}

func (jail Jail) RestoreSnapshot(snapname string) error {
    return nil
}

func (jail Jail) DeleteSnapshot(snapname string) error {
    return nil
}

func (jail Jail) PrepareHostNetworking() error {
    return nil
}

func (jail Jail) PrepareGuestNetworking() error {
    return nil
}

func (jail Jail) NetworkingStatus() string {
    return ""
}

func (jail Jail) GetPath() string {
    return ""
}

func (jail Jail) IsOnline() bool {
    return false
}

func (jail Jail) Validate() error {
    return nil
}

func (jail Jail) Persist() error {
    return nil
}

func (jail Jail) Delete() error {
    return nil
}

func (jail Jail) Archive(archivename string) error {
    return nil
}
