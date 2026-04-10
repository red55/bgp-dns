all: bgp-dnsd bgp-dnsctl

pb:
	buf generate --path proto/api


bgp-dnsd: clean pb
	go build -o ./bgp-dnsd cmd/bgp-dnsd/main.go

bgp-dnsctl: clean pb
	go build -o ./bgp-dnsctl cmd/bgp-dnsctl/main.go

clean:
	rm -f ./bgp-dns*

clean_bp:
	rm -f ./api/*.pb.go