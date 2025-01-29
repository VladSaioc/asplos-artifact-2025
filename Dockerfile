FROM amd64/golang:1.22 AS baseline

ENV BASELINE="/usr/app/baseline/bin/go"
ENV GOLF="/usr/app/golf/bin/go"

WORKDIR /usr/app

# # # # # # # # # # # # # #
#    ____         _       #
#  / ___ |       | |  __  #
# | |  |_|   _   | | / _| #
# | |      / _ \ | || |_  #
# | |  __ | | | || ||  _| #
# | | |  || | | || || |   #
# | |__| || |_| || || |   #
#  \_____| \___/ |_||_|   #
#                         #
# # # # # # # # # # # # # #

# Initial setup
RUN apt update
RUN apt install unzip
RUN (echo "Y" && cat) | apt install vim
RUN (echo "Y" && cat) | apt install python3-pip
RUN (echo "Y" && cat) | apt install python3-setuptools
RUN (echo "Y" && cat) | apt install python3-tqdm
RUN (echo "Y" && cat) | apt install sudo

FROM baseline AS golf

WORKDIR /usr/app
COPY ./baseline /usr/app/baseline
COPY ./golf /usr/app/golf

# Install baseline Go and Golf
RUN cd /usr/app/baseline/src && bash make.bash
RUN cd /usr/app/golf/src && bash make.bash

# Install Golf tester
COPY ./tester /usr/app/tester
WORKDIR /usr/app/tester

# Run microbenchmarks for coverage
RUN go run . -baseline "$BASELINE" -go "$GOLF" -report results -repeats 100 -dontmatch "(buggy|nonblocking)" -match "deadlock/(gobench|cgo-examples)"

# Run microbenchmarks for performance
RUN go run . -perf -baseline "$BASELINE" -go "$GOLF" -report results -repeats 5 -dontmatch "(buggy|nonblocking)" -match "(gobench|cgo-examples)"
