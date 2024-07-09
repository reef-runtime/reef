.PHONY: build

INPUT:=./input.c
OUTP:=../../Reef/Jobs/C_AUTOGEN_SUBMIT.bru

INPUT_STR:=$(cat $(INPUT))

#
# ID of the empty dataset.
#

DS=8b4985d2f8011f74fdf8566611b8cc8cae398ad350bc33f8b5db5bc840f92cbb

build: $(INPUT) ./req_templ0 ./req_templ1
	bash ./replace.sh $(OUTP) $(DS)
