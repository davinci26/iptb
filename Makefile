PLUGINS =
CLEAN =

PLUGINS_DIR = $(HOME)/.iptbplugins/

PLUGINS+= plugins/ipfs/local/localipfs
PLUGINS_INSTALL = $(addprefix $(PLUGINS_DIR),$(notdir $(PLUGINS)))

all: iptb plugins

iptb:
	go build

CLEAN += iptb

install: install_iptb install_plugins

install_iptb:
	go install

install_plugins: $(PLUGINS_INSTALL) $(PLUGINS_DIR)

$(PLUGINS_INSTALL): $(PLUGINS)
	cp $^.so $(@).so

plugins: $(PLUGINS)

$(PLUGINS):
	(cd $(dir $@) && go build -buildmode=plugin -o $(notdir $@).so)

CLEAN += $(addsuffix .so,$(PLUGINS))

$(PLUGINS_DIR):
	mkdir -p $@

test:
	make -C sharness all

clean:
	rm $(CLEAN)

.PHONY: all test plugins
