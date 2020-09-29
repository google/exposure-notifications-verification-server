#!/bin/sh

rm -rf go-diagrams
go run .
cd go-diagrams
dot -Tpng app.dot > diagram.png
cd -
