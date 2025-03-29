BUILDER=./.builder
RULES=go
include $(BUILDER)/rules.mk
$(BUILDER)/rules.mk:
	-go run endobit.io/builder@latest init

build::
	CGO_ENABLED=0 $(GO_BUILD) -o mopsd ./cmd

generate::
	go tool github.com/swaggo/swag/cmd/swag init -g cmd/main.go --pd --ot yaml -o .

format::
	go tool github.com/swaggo/swag/cmd/swag fmt


clean::
	rm -rf mopsd




