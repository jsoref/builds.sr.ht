# Adapted from
# https://nixos.wiki/wiki/Virtualization_in_NixOS#Building_a_NixOS_base_image
{ pkgs ? import <nixpkgs> {}, system ? builtins.currentSystem, ... }:
let
  config = (import <nixpkgs/nixos/lib/eval-config.nix> {
    inherit system;
    modules = [ {
      imports = [ ./qemu-system-configuration.nix ];
    } ];
  }).config;
in pkgs.vmTools.runInLinuxVM (
  pkgs.runCommand "nixos-qemu-image"
    {
      memSize = 768;
      preVM =
        ''
          mkdir $out
          diskImage=image.qcow2
          ${pkgs.vmTools.qemu}/bin/qemu-img create -f qcow2 $diskImage 16G
          mv closure xchg/
        '';
      postVM =
        ''
          echo compressing VM image...
          ${pkgs.vmTools.qemu}/bin/qemu-img convert -c $diskImage -O qcow2 $out/root.img.qcow2
        '';
      buildInputs = [ pkgs.utillinux pkgs.perl pkgs.parted pkgs.e2fsprogs ];
      exportReferencesGraph =
        [ "closure" config.system.build.toplevel ];
    }
    ''
      # Create the partition
      parted /dev/vda mklabel msdos
      parted /dev/vda -- mkpart primary ext4 1M -1s
      . /sys/class/block/vda1/uevent

      # Format the partition
      mkfs.ext4 -L nixos /dev/vda1
      mkdir /mnt
      mount /dev/vda1 /mnt

      for dir in dev proc sys; do
        mkdir /mnt/$dir
        mount --bind /$dir /mnt/$dir
      done

      storePaths=$(perl ${pkgs.pathsFromGraph} /tmp/xchg/closure)
      echo filling Nix store...
      mkdir -p /mnt/nix/store
      set -f
      cp -prd $storePaths /mnt/nix/store
      # The permissions will be set up incorrectly if the host machine is not running NixOS
      chown -R 0:30000 /mnt/nix/store

      mkdir -p /mnt/etc/nix
      echo 'build-users-group = ' > /mnt/etc/nix/nix.conf

      # Register the paths in the Nix database.
      printRegistration=1 perl ${pkgs.pathsFromGraph} /tmp/xchg/closure | \
          chroot /mnt ${config.nix.package.out}/bin/nix-store --load-db

      # Create the system profile to allow nixos-rebuild to work.
      chroot /mnt ${config.nix.package.out}/bin/nix-env \
          -p /nix/var/nix/profiles/system --set ${config.system.build.toplevel}

      # `nixos-rebuild' requires an /etc/NIXOS.
      mkdir -p /mnt/etc/nixos
      touch /mnt/etc/NIXOS

      # `switch-to-configuration' requires a /bin/sh
      mkdir -p /mnt/bin
      ln -s ${config.system.build.binsh}/bin/sh /mnt/bin/sh

      # Generate the GRUB menu.
      chroot /mnt ${config.system.build.toplevel}/bin/switch-to-configuration boot

      umount /mnt/{proc,dev,sys}
      umount /mnt
    ''
)

