#!/bin/bash

for f in $(du -a . | awk '{print $2}' | grep '\.go$' | grep -v '/vendor/' | grep -v 'test/mock' | grep -v '\.pb.go$' | grep -v 'explorer/idl'); do
  if !(grep Copyright $f);then
    cat misc/scripts/license_header.txt $f > $f.new
    mv $f.new $f
  fi
done
