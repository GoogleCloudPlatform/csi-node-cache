# Copyright 2024 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This should be build from the repo root.

ARG VERSION

FROM golang:1.22 AS builder
WORKDIR /src
COPY . .
RUN go build -ldflags "-extldflags=static -X main.driverVersion=$VERSION" ./cmd/driver

FROM debian:12 AS debian
# google_nvme_id script depends on the following packages: nvme-cli, xxd, bash
RUN apt update && apt install -y \
  mount bash mdadm util-linux e2fsprogs nvme-cli xxd

RUN /usr/bin/ldd /bin/bash
RUN /usr/bin/ldd /bin/sh

# Since we're leveraging apt to pull in dependencies, we use
# `gcr.io/distroless/base` because it includes glibc.
FROM gcr.io/distroless/base-debian12 AS distroless

COPY --from=builder /src/driver /
COPY --from=debian /bin/mount /bin/umount /sbin/mdadm /bin/
COPY --from=debian /sbin/blkid /sbin/blkid
COPY --from=debian /sbin/blockdev /sbin/blockdev
COPY --from=debian /sbin/dumpe2fs /sbin/dumpe2fs
COPY --from=debian /sbin/e2fsck /sbin/e2fsck
COPY --from=debian /sbin/fsck /sbin/fsck
COPY --from=debian /sbin/fsck.ext2 /sbin/
COPY --from=debian /sbin/fsck.ext3 /sbin/
COPY --from=debian /sbin/fsck.ext4 /sbin/
COPY --from=debian /etc/mke2fs.conf /etc/mke2fs.conf
COPY --from=debian /sbin/mkfs /sbin/
COPY --from=debian /sbin/mke2fs /sbin/
# These are symlinks to the same thing, but I can't figure out how to make
# a symlink without pulling /bin/sh into the container.
COPY --from=debian /sbin/mkfs.ext2 /sbin/mkfs.ext3 /sbin/mkfs.ext4 /sbin/

COPY --from=debian \
    /lib/x86_64-linux-gnu/libselinux.so.* \
    /lib/x86_64-linux-gnu/libblkid.so.* \
    /lib/x86_64-linux-gnu/libpcre2-8.so.* \
    /lib/x86_64-linux-gnu/libmount.so.* \
    /lib/x86_64-linux-gnu/libudev.so.* \
    /lib/x86_64-linux-gnu/libcom_err.so.* \
    /lib/x86_64-linux-gnu/libe2p.so.* \
    /lib/x86_64-linux-gnu/libext2fs.so.* \
    /lib/x86_64-linux-gnu/libuuid.so.* \
    /lib/x86_64-linux-gnu/

FROM distroless AS check

COPY --from=debian /usr/bin/ldd /usr/bin/find /usr/bin/xargs /usr/bin/
COPY --from=debian /bin/sh /bin/bash /bin/grep /bin/
COPY --from=builder /src/hack/print-missing-deps.sh /print-missing-deps.sh

COPY --from=debian \
    /lib/x86_64-linux-gnu/libtinfo.so.* \
    /lib/x86_64-linux-gnu/

SHELL ["/bin/bash", "-c"]
RUN /print-missing-deps.sh

# Final output with entrypoint
FROM distroless

ENTRYPOINT ["/driver"]
