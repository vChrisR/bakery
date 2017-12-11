package main

import (
	"fmt"
	"io/ioutil"
	"sync"
)

var nfsExportMutex = &sync.Mutex{}

func regenNfsExports(piInv piInventory) error {
	nfsExportMutex.Lock()
	defer nfsExportMutex.Unlock()

	piList, err := piInv.ListOven()
	if err != nil {
		return err

	}

	exportsContent := ""
	for _, pi := range piList {
		exportsContent = exportsContent + fmt.Sprintf("%v *(rw,sync,no_subtree_check,no_root_squash)\n", pi.GetRootLocation())
	}

	fmt.Println("Generated new exports file:\n" + exportsContent)
	err = ioutil.WriteFile("/etc/exports", []byte(exportsContent), 0644)

	//TODO: run exportfs -a
	return err
}