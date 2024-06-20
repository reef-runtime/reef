.PHONY: buld

INPUT:=./input.c
OUTP:=../../Reef/Jobs/C_AUTOGEN_SUBMIT.bru

INPUT_STR:=$(cat $(INPUT))

build: $(INPUT) ./req_templ0 ./req_templ1
	bash ./replace.sh $(OUTP)
