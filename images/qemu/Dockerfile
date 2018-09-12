FROM debian:sid

RUN apt-get -y update && apt-get -y upgrade && apt-get install -y \
    build-essential pkg-config curl xz-utils

RUN curl -O https://download.qemu.org/qemu-3.0.0.tar.xz \
    && tar xvf qemu-3.0.0.tar.xz

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

RUN cd qemu-3.0.0 && ./configure \
    --prefix=/ \
    --static \
    --python=/usr/bin/python2 \
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
    --target-list="x86_64-softmmu"

RUN cd qemu-3.0.0 && make && make install

FROM scratch
COPY --from=0 /bin/qemu-system-x86_64 /bin/
COPY --from=0 /share/qemu/ /share/qemu/
COPY --from=0 /lib/x86_64-linux-gnu/libnss_dns-2.24.so /lib/x86_64-linux-gnu/
COPY --from=0 /lib/x86_64-linux-gnu/libnss_dns.so.2 /lib/x86_64-linux-gnu/
COPY --from=0 /etc/nsswitch.conf /etc/

CMD ["/bin/qemu-system-x86_64"]
