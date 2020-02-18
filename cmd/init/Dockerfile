FROM ubuntu

	RUN apt update && apt install -y gcc
	ADD ./init.c /init.c
	RUN gcc -O2 -static -o /init /init.c
