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
FROM golang:1.22 AS builder
WORKDIR /src
COPY . .
RUN go build -ldflags "-extldflags=static" ./cmd/controller

# I'd like to use gcr.io/distroless/static:latest, but I ran into a
# weird problem where no executables were found. So I copied the
# working example from the driver using base-debian12, with the
# library check flow run to confirm that the images works correctly.
FROM gcr.io/distroless/base-debian12 AS distroless
COPY --from=builder /src/controller /

FROM debian:12 AS debian
RUN apt update && apt install -y bash

FROM distroless AS check
COPY --from=debian /usr/bin/ldd /usr/bin/find /usr/bin/xargs /usr/bin/
COPY --from=debian /bin/sh /bin/bash /bin/grep /bin/
COPY --from=builder /src/hack/print-missing-deps.sh /print-missing-deps.sh
COPY --from=debian \
    /lib/x86_64-linux-gnu/libtinfo.so.* \
    /lib/x86_64-linux-gnu/libselinux.so.* \
    /lib/x86_64-linux-gnu/libpcre2-8.so.* \
    /lib/x86_64-linux-gnu/

SHELL ["/bin/bash", "-c"]
RUN /print-missing-deps.sh

FROM distroless
ENTRYPOINT ["/controller"]
