
# disk-scheduler

Schedule PD disks in the correct CoreOS instances when requested by the services.


### Usage

Use this app to transfer the disk from another instance if it's not already attached to the correct one.

First you will have to build an image using the provided `Dockerfile.example` and push it to registry you want (Google Container Registry or Docker Hub for example).

Then use the app in the `.service` file before starting the container and the volume.

Example `.service` file using the `disk-name` disk and mounting it in `/foodir`:

```
[Unit]
Description=Foo service description
Requires=docker.service
After=docker.service

[Service]
Restart=always
TimeoutStartSec=0
RestartSec=10
KillMode=none
EnvironmentFile=/etc/environment

# Transfer the disk to our instance, and mount it
ExecStartPre=/usr/bin/docker pull myuser/disk-scheduler
ExecStartPre=/usr/bin/docker run --rm -e DISK=disk-name myuser/disk-scheduler
ExecStartPre=-/bin/umount /foodir
ExecStartPre=-/bin/rmdir /foodir
ExecStartPre=/bin/mkdir /foodir
ExecStartPre=/usr/share/oem/google-startup-scripts/safe_format_and_mount -f ext4 \
  /dev/disk/by-id/scsi-0Google_PersistentDisk_disk-name /foodir

# Download the real image and kill any old instance of it in the machine
ExecStartPre=docker pull myuser/real-image
ExecStartPre=-/usr/bin/docker kill real-container
ExecStartPre=-/usr/bin/docker rm real-container

# Run our app
ExecStart=/usr/bin/docker run --name real-container -v /foodir:/data myuser/real-image

# Remove the container once we have finished
ExecStop=/usr/bin/docker stop real-container
ExecStop=/usr/bin/docker rm real-container

# Remove the disk cleanly when the container finishes
ExecStop=/bin/umount /foodir
ExecStop=/bin/rmdir /foodir
```
