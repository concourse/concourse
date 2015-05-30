all: out

out:
	RUBYLIB=${PWD}/lib anatomy -i concourse.any -o out
