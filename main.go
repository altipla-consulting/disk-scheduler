package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/juju/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	compute "google.golang.org/api/compute/v1"
)

var (
	flagDisk = flag.String("disk", "", "disk name to attach to the instance")
)

func main() {
	if err := runSafe(); err != nil {
		log.Println(errors.ErrorStack(err))
	}
}

func runSafe() error {
	flag.Parse()

	if *flagDisk == "" {
		return errors.NotValidf("--disk flag is required")
	}

	log.Println(" [*] Attaching disk", *flagDisk, "to the instance...")

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
	oldInstance, err := findAttachedInstance(service, project, zone)
	if err != nil {
		return errors.Trace(err)
	}
	if oldInstance != instanceName {
		if oldInstance != "" {
			log.Println(" > Deattaching disk from instance:", oldInstance)
			if err := detachDisk(service, project, zone, oldInstance); err != nil {
				return errors.Trace(err)
			}
		}

		log.Println(" > Attach disk to this instance...")
		if err := attachDisk(service, project, zone, instanceName); err != nil {
			return errors.Trace(err)
		}
	}

	log.Println(" [*] Disk attached successfully!")

	return nil
}

func getMetadata(path string) (string, error) {
	u := fmt.Sprintf("http://metadata.google.internal/computeMetadata/v1/%s", path)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", errors.Trace(err)
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := http.DefaultClient.Do(req)
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
	resp, err := service.Instances.DetachDisk(project, zone, instance, *flagDisk).Do()
	if err != nil {
		return errors.Trace(err)
	}

	for true {
		operationResult, err := service.ZoneOperations.Get(project, zone, resp.Name).Do()
		if err != nil {
			return errors.Trace(err)
		}

		if operationResult.Error != nil {
			return errors.Errorf("operation error, see: %s", resp.Name)
		}

		if operationResult.Status == "DONE" {
			break
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}

func attachDisk(service *compute.Service, project, zone, instance string) error {
	disk := &compute.AttachedDisk{
		DeviceName: *flagDisk,
		Source:     fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/disks/%s", project, zone, *flagDisk),
	}
	resp, err := service.Instances.AttachDisk(project, zone, instance, disk).Do()
	if err != nil {
		return errors.Trace(err)
	}

	for true {
		operationResult, err := service.ZoneOperations.Get(project, zone, resp.Name).Do()
		if err != nil {
			return errors.Trace(err)
		}

		if operationResult.Error != nil {
			return errors.Errorf("operation error, see: %s", resp.Name)
		}

		if operationResult.Status == "DONE" {
			break
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}
