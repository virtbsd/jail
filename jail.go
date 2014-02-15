package jail

/*
 * The jail.Jail object implements the VirtualMachine interface
 */

import (
    "github.com/coopernurse/gorp"
)

type Jail struct {
    UUID string
    Name string
    HostName string
    Options map[string]string `db:"-"`
    CreateDate int
    ModificationDate int

    Dirty bool `db:"-"`
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

func Archive(archivename string) error {
    return nil
}
