package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

type bakeformInventory interface {
	Load() error //loads alls images from the image folder
	List() BakeformList
	UnmountAll() error
	ListHandler(w http.ResponseWriter, r *http.Request)
	UploadHandler(w http.ResponseWriter, r *http.Request)
	DeleteHandler(w http.ResponseWriter, r *http.Request)
}

type BakeformInventory struct {
	folder     string
	mountRoot  string
	nfs        fileBackend
	Content    BakeformList
	kpartxPath string
}

func newBakeformInventory(folder, mountRoot string, nfs fileBackend, kpartxPath string) (bakeformInventory, error) {
	if mountRoot == "" || folder == "" {
		return &BakeformInventory{}, fmt.Errorf("Please set IMAGE_FOLDER and IMAGE_MOUNT_ROOT en vars.")
	}

	newInv := &BakeformInventory{
		folder:     folder,
		mountRoot:  mountRoot,
		nfs:        nfs,
		kpartxPath: kpartxPath,
	}

	err := newInv.Load()
	if err != nil {
		return &BakeformInventory{}, err
	}

	return newInv, err
}

func (i *BakeformInventory) Load() error {
	imgFiles, err := filepath.Glob(i.folder + "/*.img")
	if err != nil {
		return err
	}

	list := make(BakeformList)

	for _, img := range imgFiles {
		nameParts := strings.Split(img, "/")
		name := strings.Replace(nameParts[len(nameParts)-1], ".img", "", 1)

		log.Printf("Loading image %v\n", name)
		bf := &Bakeform{
			Name:         name,
			Location:     img,
			mountRoot:    i.mountRoot,
			fb:           i.nfs,
			bootLocation: path.Join(i.nfs.GetBootRoot(), name),
			kpartxPath:   i.kpartxPath,
		}

		_, err := os.Stat(bf.bootLocation)
		if os.IsNotExist(err) {
			err := bf.mount()
			if err != nil {
				return err
			}

			_, err = i.nfs.CopyBootFolder(bf.MountedOn[0]+"/", name)
			bf.unmount()
			if err != nil {
				return err
			}
		}

		list[name] = bf
	}

	i.Content = list

	return nil
}

func (i *BakeformInventory) List() BakeformList {
	return i.Content
}

func (i *BakeformInventory) ListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	jsonBytes, err := json.Marshal(i.Content)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Write(jsonBytes)
}

func (i *BakeformInventory) UploadHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	urlvars := mux.Vars(r)
	name := urlvars["name"]
	filepath := i.folder + "/" + name + ".img"

	log.Println("Receiving upload: " + filepath)

	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		log.Printf("Error creating image file: %v\n", err)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(err.Error()))
		return
	}

	_, err = io.Copy(file, r.Body)
	if err != nil {
		log.Printf("Error saving image: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	err = i.Load()
	if err != nil {
		log.Printf("Error loading images: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	jsonBytes, _ := json.Marshal(i.Content[name])
	w.WriteHeader(http.StatusCreated)
	w.Write(jsonBytes)
}

func (i *BakeformInventory) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	urlvars := mux.Vars(r)
	name := urlvars["name"]

	bf, exists := i.Content[name]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Bakeform not found"))
		return
	}

	err := bf.Delete()
	if err != nil {
		log.Printf("Error deleting bakeform: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	err = i.Load()
	if err != nil {
		log.Printf("Error Reloading bakerforms: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}

func (i *BakeformInventory) UnmountAll() error {
	for _, b := range i.Content {
		b.unmount()
	}
	return nil
}
