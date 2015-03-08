VERSION?=0.0.0

all: install build

install:
	raco pkg install --auto -j 4 scribble || true

build:
	raco scribble ++arg --version ++arg ${VERSION} --htmls concourse.scrbl

