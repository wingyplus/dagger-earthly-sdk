#!/bin/sh

name=$1
# rm -rf $name
mkdir -p "${name}/earthly-sdk"
cp -r runtime "${name}/earthly-sdk/runtime"
cp -r .dagger "${name}/earthly-sdk/.dagger"
cp -r dagger.json "${name}/earthly-sdk/dagger.json"
cd $name
dagger init --sdk=./earthly-sdk --source=. --name=$name

