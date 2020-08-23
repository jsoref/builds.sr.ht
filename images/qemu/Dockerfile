FROM debian:sid

RUN apt-get -y update --fix-missing && apt-get -y upgrade && apt-get install -y \
    build-essential pkg-config curl xz-utils

RUN curl -O https://download.qemu.org/qemu-4.1.0.tar.xz \
    && tar xvf qemu-4.1.0.tar.xz

RUN apt-get install -y \
    libaio-dev \
    libcap-dev \
    libcap-ng-dev \
    liblzo2-dev \
    texinfo \
    vde2 \
    zlib1g-dev \
    libpixman-1-dev \
    libglib2.0-dev

RUN cd qemu-4.1.0 && ./configure \
    --prefix=/ \
    --static \
    --python=/usr/bin/python3 \
    --audio-drv-list="" \
    --disable-docs \
    --disable-debug-info \
    --disable-opengl \
    --disable-virglrenderer \
    --disable-vte \
    --disable-gtk \
    --disable-sdl \
    --disable-bluez \
    --disable-spice \
    --disable-vnc \
    --disable-curses \
    --disable-xen \
    --disable-smartcard \
    --disable-libnfs \
    --disable-libusb \
    --disable-glusterfs \
    --disable-tools \
    --disable-werror \
    --target-list="x86_64-softmmu,i386-softmmu,aarch64-softmmu,arm-softmmu,ppc64-softmmu,s390x-softmmu,riscv64-softmmu,mips-softmmu,mipsel-softmmu,mips64el-softmmu"

RUN cd qemu-4.1.0 && make && make install

FROM scratch
COPY --from=0 /bin/qemu-system-x86_64 /bin/
COPY --from=0 /bin/qemu-system-i386 /bin/
COPY --from=0 /bin/qemu-system-aarch64 /bin/
COPY --from=0 /bin/qemu-system-arm /bin/
COPY --from=0 /bin/qemu-system-ppc64 /bin/
COPY --from=0 /bin/qemu-system-s390x /bin/
COPY --from=0 /bin/qemu-system-mips /bin/
COPY --from=0 /bin/qemu-system-mipsel /bin/
COPY --from=0 /bin/qemu-system-mips64el /bin/
COPY --from=0 /share/qemu/ /share/qemu/
# It is safe and recommended to update the libnss_dns version if you
# don't have this version on your system.
# Please send a patch in as well! :)
COPY --from=0 /lib/x86_64-linux-gnu/libnss_dns-2.31.so /lib/x86_64-linux-gnu/
COPY --from=0 /lib/x86_64-linux-gnu/libnss_dns.so.2 /lib/x86_64-linux-gnu/
COPY --from=0 /etc/nsswitch.conf /etc/

CMD ["/bin/qemu-system-x86_64"]
