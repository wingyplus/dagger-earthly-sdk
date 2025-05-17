# 🌍🚀 Dagger Earthly SDK  

**Run Earthly target as a Dagger function**

## 🚀 Overview  

This SDK enables you to run Earthly task with Dagger without spending too much effort.

## 🧠 Key Features  

- **Initialize and play**: Just initialize a module into your project and calling it!
- **Automatic export a container**: The SDK converts your task into a Dagger Container when it declares `SAVE IMAGE` instruction. 

## 📌 Getting Started 

First, initialize module by `dagger init`:

```
$ dagger init --sdk=github.com/wingyplus/dagger-earthly-sdk --source=./potato potato
```

Once initialized, the `Earthfile` will get generated alongs with `dagger.json`:

```
$ ls -l
dagger.json
Earthfile
$ cat Earthfile
VERSION 0.8

# echo-container say anything.
echo-container:
  ARG --required STRING_ARG
  FROM alpine:latest
  RUN echo ${STRING_ARG} > /hello.txt
  SAVE IMAGE earthly-dagger-container
```

Now, we can execute a function with `dagger call`:

```
$ dagger call echo-container --string-arg='Hello' file --path=/hello.txt contents
✔ connect 0.5s
✔ load module 23.1s
✔ parsing command line arguments 0.0s

✔ potato: Potato! 0.7s
∅ .echoContainer(stringArg: "Hello"): Container! 13.6s
∅ .file(path: "/hello.txt"): File! 0.0s
∅ .contents: String! 0.0s
┃ Hello                                                                                                 

Hello
```
