VERSION?=0.0.0

all:
	raco pkg install --auto -j 4 scribble || true
	raco scribble ++arg ${VERSION} --htmls concourse.scrbl

