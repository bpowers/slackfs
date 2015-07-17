
MNT ?= /tmp/slack
TOKEN_PATH ?= ~/.slack-token
INFO ?= info.json

all: run

unmount:
	fusermount -u $(MNT) 2>/dev/null || true

build:
	go build

run: build
	./slackfs $(FLAGS) $(MNT)

run-offline: build
	./slackfs $(FLAGS) -offline $(INFO) $(MNT)

update-offline-info:
	curl "https://slack.com/api/rtm.start?token=$(shell cat $(TOKEN_PATH) | cut -d ' ' -f 3)" | pretty >info.json

count:
	cat $(MNT)/users/by-name/**/presence | sort | uniq -c

clean:
	rm -f ./slackfs *~

.PHONY: all unmount build run run-offline count clean
