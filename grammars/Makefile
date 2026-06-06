CFLAGS  = -O2 -shared -fPIC -Iinclude
OUTDIR  = dist
SRCDIR  = src

.PHONY: all clean php ts py vue

all: $(OUTDIR)/php.so $(OUTDIR)/ts.so $(OUTDIR)/py.so $(OUTDIR)/vue.so

$(OUTDIR):
	mkdir -p $(OUTDIR)

# PHP grammar
$(OUTDIR)/php.so: $(OUTDIR) $(SRCDIR)/php/php/src/parser.c
	$(CC) $(CFLAGS) \
	  -I$(SRCDIR)/php/php/src \
	  $(SRCDIR)/php/php/src/parser.c \
	  $(SRCDIR)/php/php/src/scanner.c \
	  -o $@

# TypeScript grammar
$(OUTDIR)/ts.so: $(OUTDIR) $(SRCDIR)/typescript/typescript/src/parser.c
	$(CC) $(CFLAGS) \
	  -I$(SRCDIR)/typescript/typescript/src \
	  $(SRCDIR)/typescript/typescript/src/parser.c \
	  $(SRCDIR)/typescript/typescript/src/scanner.c \
	  -o $@

# Python grammar
$(OUTDIR)/py.so: $(OUTDIR) $(SRCDIR)/python/src/parser.c
	$(CC) $(CFLAGS) \
	  -I$(SRCDIR)/python/src \
	  $(SRCDIR)/python/src/parser.c \
	  $(SRCDIR)/python/src/scanner.c \
	  -o $@

# Vue grammar (C++ scanner -- compile steps separated so -w only hits generated parser.c)
$(OUTDIR)/vue.so: $(OUTDIR) $(SRCDIR)/vue/src/parser.c $(SRCDIR)/vue/src/scanner.cc
	$(CC) -O2 -fPIC -w \
	  -I$(SRCDIR)/vue/src \
	  -c $(SRCDIR)/vue/src/parser.c \
	  -o $(OUTDIR)/vue_parser.o
	$(CXX) -O2 -fPIC \
	  -I$(SRCDIR)/vue/src \
	  -c $(SRCDIR)/vue/src/scanner.cc \
	  -o $(OUTDIR)/vue_scanner.o
	$(CXX) -shared -o $@ $(OUTDIR)/vue_parser.o $(OUTDIR)/vue_scanner.o

# Fetch grammar sources (requires git)
fetch:
	mkdir -p $(SRCDIR)
	@if [ ! -d $(SRCDIR)/php ]; then \
	  git clone --depth 1 https://github.com/tree-sitter/tree-sitter-php $(SRCDIR)/php; fi
	@if [ ! -d $(SRCDIR)/typescript ]; then \
	  git clone --depth 1 https://github.com/tree-sitter/tree-sitter-typescript $(SRCDIR)/typescript; fi
	@if [ ! -d $(SRCDIR)/python ]; then \
	  git clone --depth 1 https://github.com/tree-sitter/tree-sitter-python $(SRCDIR)/python; fi
	@if [ ! -d $(SRCDIR)/vue ]; then \
	  git clone --depth 1 https://github.com/ikatyang/tree-sitter-vue $(SRCDIR)/vue; fi

clean:
	rm -rf $(OUTDIR) $(SRCDIR)
