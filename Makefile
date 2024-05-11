all: bgp-dns

bgp-dns: clean
	go build -o ./bgp-dnsd cmd/bgp-dnsd/main.go

clean:
	rm -f ./bgp-dnsd