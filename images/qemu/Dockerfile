FROM alpine:latest

RUN apk update && apk upgrade && \
    apk add alsa-lib-dev bison curl curl-dev flex glib-dev glib-static \
    libaio-dev libcap-dev libcap-ng-dev linux-headers lzo-dev paxmark \
    texinfo util-linux-dev vde2-dev xfsprogs-dev zlib-dev pixman-dev \
    xz alpine-sdk

# pixman
# https://bugs.alpinelinux.org/issues/8376
RUN curl -O https://www.cairographics.org/releases/pixman-0.34.0.tar.gz \
    && tar xf pixman-0.34.0.tar.gz

COPY ./pixman-patches /pixman-patches

RUN cd pixman-0.34.0 && /pixman-patches/apply

RUN cd pixman-0.34.0 && ./configure \
    --prefix=/usr \
    --enable-static \
    --disable-openmp \
    --disable-arm-iwmmxt

RUN cd pixman-0.34.0 && make && make install

# qemu
RUN curl -O https://download.qemu.org/qemu-2.11.0.tar.xz \
    && tar xf qemu-2.11.0.tar.xz

COPY ./qemu-patches /qemu-patches

RUN cd qemu-2.11.0 && /qemu-patches/apply && \
    sed -i 's/^VL_LDFLAGS=$/VL_LDFLAGS=-Wl,-z,execheap/' Makefile.target

RUN cd qemu-2.11.0 && ./configure \
    --prefix=/ \
    --static \
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

RUN cd qemu-2.11.0 && make && make install

FROM scratch
COPY --from=0 /bin/qemu-system-x86_64 /bin/
COPY --from=0 /share/qemu/ /share/qemu/

CMD ["/bin/qemu-system-x86_64"]
