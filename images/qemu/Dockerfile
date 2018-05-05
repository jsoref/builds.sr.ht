FROM debian:sid

RUN apt-get -y update && apt-get -y upgrade && apt-get install -y \
    build-essential pkg-config curl xz-utils

RUN curl -O https://download.qemu.org/qemu-2.12.0.tar.xz \
    && tar xf qemu-2.12.0.tar.xz

RUN apt-get install -y \
    python \
    libaio-dev \
    libcap-dev \
    libcap-ng-dev \
    liblzo2-dev \
    texinfo \
    vde2 \
    zlib1g-dev \
    libpixman-1-dev \
    libglib2.0-dev

RUN cd qemu-2.12.0 && ./configure \
    --prefix=/ \
    --static \
    --python=/usr/bin/python2 \
    --audio-drv-list="" \
    --disable-docs \
    --disable-debug-info \
    --disable-bsd-user \
    --disable-werror \
    --disable-sdl \
    --disable-xen \
    --disable-guest-agent \
    --disable-modules \
    --disable-gnutls \
    --disable-nettle \
    --disable-gcrypt \
    --disable-gtk \
    --disable-vte \
    --disable-curses \
    --disable-vnc \
    --disable-bluez \
    --disable-hax \
    --disable-rdma \
    --disable-spice \
    --disable-libnfs \
    --disable-smartcard \
    --disable-libusb \
    --disable-glusterfs \
    --disable-numa \
    --disable-libssh2 \
    --disable-tools \
    --disable-vxhs \
    --disable-opengl \
    --disable-linux-user \
    --disable-virglrenderer \
    --target-list="x86_64-softmmu"

RUN cd qemu-2.12.0 && make && make install

FROM scratch
COPY --from=0 /bin/qemu-system-x86_64 /bin/
COPY --from=0 /share/qemu/ /share/qemu/

CMD ["/bin/qemu-system-x86_64"]
