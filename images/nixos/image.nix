{ pkgs ? import <nixpkgs> {}, ... }:

let
  makeDiskImage = import <nixpkgs/nixos/lib/make-disk-image.nix>;
  evalConfig = import <nixpkgs/nixos/lib/eval-config.nix>;
  config = (evalConfig {
    modules = [ (import ./qemu-system-configuration.nix) ];
  }).config;
in
  makeDiskImage {
    inherit pkgs config;
    lib = pkgs.lib;
    diskSize = 16000;
    format = "qcow2-compressed";
  }

