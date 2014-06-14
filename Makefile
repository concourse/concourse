all: ci-image

# TODO build input, not statically config'd
testflight-raw-resource.cid: resource-images/raw/Dockerfile
	docker build -t concourse/testflight-raw-resource --rm resource-images/raw
	docker run --cidfile=testflight-raw-resource.cid concourse/testflight-raw-resource echo

testflight-ubuntu.cid: staticregistry/Dockerfile
	docker build -t concourse/testflight-ubuntu --rm staticregistry
	docker run --cidfile=testflight-ubuntu.cid concourse/testflight-ubuntu echo

images:
	mkdir images

images/ubuntu.tar: images testflight-ubuntu.cid
	docker export `cat testflight-ubuntu.cid` > images/ubuntu.tar
	docker rm `cat testflight-ubuntu.cid`
	rm testflight-ubuntu.cid

images/raw-resource.tar: images testflight-raw-resource.cid
	docker export `cat testflight-raw-resource.cid` > images/raw-resource.tar
	docker rm `cat testflight-raw-resource.cid`
	rm testflight-raw-resource.cid

ci-image: images/ubuntu.tar images/raw-resource.tar
	docker build -t concourse/testflight --rm .
	rm -rf images/
