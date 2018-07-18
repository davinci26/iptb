PLUGINS =
CLEAN =

all: iptb

iptb:
	go build
.PHONY: iptb # let go figure it out

CLEAN += iptb

install: install_iptb

install_iptb:
	go install

test:
	make -C sharness all

clean:
	rm $(CLEAN)

.PHONY: all test plugins
