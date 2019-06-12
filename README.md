Common
======

> Common util function in go.


## Table of Contents

- [Usage](#usage)
- [API](#api)
- [Mockgen](#mockgen)
- [GitCZ](#gitcz)
- [Contribute](#contribute)
- [License](#license)

## Mockgen

Install **mockgen** : `go get github.com/golang/mock/mockgen`

How to use?

- source： 指定接口文件
- destination: 生成的文件名
- package:生成文件的包名
- imports: 依赖的需要import的包
- aux_files:接口文件不止一个文件时附加文件
- build_flags: 传递给build工具的参数

Eg.`mockgen -destination mock/mock_common.go -package common -source common.go`

## GitCZ

**Note**: Please use command `npm install` if you are the first time to use `git cz` in this repo.

## Contribute

PRs are welcome!

Small note: If editing the Readme, please conform to the [standard-readme](https://github.com/RichardLitt/standard-readme) specification.

## License

Copyright © 2016-2019 Hangzhou Qulian Technology Co., Ltd.