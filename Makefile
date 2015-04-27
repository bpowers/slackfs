
MNT := /tmp/slack
TOKEN := $(shell cat ~/.slack | cut -d ' ' -f 3)

all: run

unmount:
	sudo umount $(MNT) 2>/dev/null || true

build:
	go build

run: unmount build
	./slackfs  -token $(TOKEN) $(MNT)

run-offline: unmount build
	./slackfs -offline info.json -token $(TOKEN) $(MNT)

count:
	cat /tmp/slack/**/presence | sort | uniq -c

clean:
	rm -f ./slackfs *~

.PHONY: all unmount build run run-offline count clean
