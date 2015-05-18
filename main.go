package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/juju/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	compute "google.golang.org/api/compute/v1"
)

var (
	flagDisk = flag.String("disk", "", "disk name to attach to the instance")
	flagPath = flag.String("path", "", "path in the current instance to mount the disk")
)

func main() {
	if err := runSafe(); err != nil {
		log.Println(errors.ErrorStack(err))
	}
}

func runSafe() error {
	flag.Parse()

	if *flagDisk == "" || *flagPath == "" {
		return errors.NotValidf("--disk and --path are required")
	}

	log.Println(" [*] Attaching disk", *flagDisk, "to the instance in path: ", *flagPath)

	client := &http.Client{
		Transport: &oauth2.Transport{
			Source: google.ComputeTokenSource(""),
		},
	}
	service, err := compute.New(client)
	if err != nil {
		return errors.Trace(err)
	}

	log.Println(" > Get metadata...")
	project, err := getMetadata("project/project-id")
	if err != nil {
		return errors.Trace(err)
	}

	rawZone, err := getMetadata("instance/zone")
	if err != nil {
		return errors.Trace(err)
	}
	parts := strings.Split(rawZone, "/")
	zone := parts[len(parts)-1]

	rawInstanceName, err := getMetadata("instance/hostname")
	if err != nil {
		return errors.Trace(err)
	}
	parts = strings.Split(rawInstanceName, ".")
	instanceName := parts[0]

	log.Println(" > Check disk name is correct...")
	if err := checkDiskExists(service, project, zone); err != nil {
		return errors.Trace(err)
	}

	log.Println(" > Check if there is another instance with the disk...")
	instance, err := findAttachedInstance(service, project, zone)
	if err != nil {
		return errors.Trace(err)
	}
	if instance != "" {
		log.Println(" > Deattaching disk from instance:", instance)
		if err := detachDisk(service, project, zone, instance); err != nil {
			return errors.Trace(err)
		}
	}

	log.Println(" > Attach disk to this instance...")
	if err := attachDisk(service, project, zone, instanceName); err != nil {
		return errors.Trace(err)
	}

	log.Println(" [*] Disk attached successfully!")

	return nil
}

func getMetadata(path string) (string, error) {
	u := fmt.Sprintf("http://metadata.google.internal/computeMetadata/v1/%s", path)

	resp, err := http.Get(u)
	if err != nil {
		return "", errors.Trace(err)
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Trace(err)
	}

	return string(content), nil
}

func checkDiskExists(service *compute.Service, project, zone string) error {
	_, err := service.Disks.Get(project, zone, *flagDisk).Do()
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func findAttachedInstance(service *compute.Service, project, zone string) (string, error) {
	instances, err := service.Instances.List(project, zone).Do()
	if err != nil {
		return "", errors.Trace(err)
	}
	for _, instance := range instances.Items {
		for _, disk := range instance.Disks {
			if disk.DeviceName == *flagDisk {
				return instance.Name, nil
			}
		}
	}

	return "", nil
}

func detachDisk(service *compute.Service, project, zone, instance string) error {
	_, err := service.Instances.DetachDisk(project, zone, instance, *flagDisk).Do()
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func attachDisk(service *compute.Service, project, zone, instance string) error {
	disk := &compute.AttachedDisk{
		DeviceName: *flagDisk,
		Source:     fmt.Sprintf("https://content.googleapis.com/compute/v1/projects/%s/zones/%s/disks/%s", project, zone, *flagDisk),
	}
	_, err := service.Instances.AttachDisk(project, zone, instance, disk).Do()
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}
