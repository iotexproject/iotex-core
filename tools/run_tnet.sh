#!/bin/bash

for i in `seq 0 4`; do
  node=$1$i
  docker run --rm --network host stonevan $node &
done

exit 0