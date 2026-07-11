![Anglerphish logo](static/images/gophish_purple.png)

<div align="center">
<h1>Anglerphish</h1>
</div>

Anglerphish is an enhanced, feature-rich fork of [Gophish](https://github.com/gophish/gophish) aimed at providing more flexible campaign management, expanded phishing vectors, improved reporting capabilities, and numerous quality‑of‑life enhancements.

See also the Medium [article](https://medium.com/@gpetro/anglerphish-6dc3e5520242).

---

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Features and Enhancements](#features-and-enhancements)
- [Visual Previews](#visual-previews)
- [A fork based on original Gophish v0.12.1:](#a-fork-based-on-original-gophish-v0121)
  - [Gophish: Open-Source Phishing Toolkit](#gophish-open-source-phishing-toolkit)
  - [Install](#install)
  - [Building From Source](#building-from-source)
  - [Setup](#setup)
  - [Documentation](#documentation)
  - [Issues](#issues)
  - [License](#license)

---

## Features and Enhancements

See [FEATURES.md](FEATURES.md) for the full list of features and enhancements.

## Visual Previews

![1](static/images/1.gif)
![2](static/images/2.gif)
![3](static/images/3.gif)
![4](static/images/4.gif)

## A fork based on original Gophish v0.12.1:

![Build Status](https://github.com/geopetro/anglerphish/workflows/CI/badge.svg) [![GoDoc](https://godoc.org/github.com/gophish/gophish?status.svg)](https://godoc.org/github.com/gophish/gophish)

### Gophish: Open-Source Phishing Toolkit

[Gophish](https://getgophish.com) is an open-source phishing toolkit designed for businesses and penetration testers. It provides the ability to quickly and easily setup and execute phishing engagements and security awareness training.

### Install

Installation of Anglerphish remains dead-simple - just download and extract the zip containing the [release for your system](https://github.com/geopetro/anglerphish/releases/), and run the binary. Anglerphish has also binary releases for Windows, Mac, and Linux platforms.

### Building From Source

To build Anglerphish from source, simply run ```git clone https://github.com/geopetro/anglerphish.git``` and ```cd``` into the project source directory. Then, run ```go build```. After this, you should have a binary called ```gophish``` in the current directory.

### Setup
After running the Gophish binary, open an Internet browser to https://localhost:3333 and login with the default username and password listed in the log output.
e.g.
```
time="2020-07-29T01:24:08Z" level=info msg="Please login with the username admin and the password 4304d5255378177d"
```

### Documentation

Documentation for Anglerphish - Documentation section includes several Anglerphish additions such as newly added API Endpoints.

Documentation of the original gophish can be found on the official [site](http://getgophish.com/documentation).

### Issues

🐞 Found a bug? Feel free to [file an issue](https://github.com/geopetro/anglerphish/releases/issues/new) — feedback is always welcome!

### License
```
MIT License

Copyright (c) 2013–2020 Jordan Wright
Copyright (c) 2025–2026 George Petropoulos

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

----------------------------------------------------------------
Fork Attribution
----------------------------------------------------------------

Anglerphish is an enhanced fork of Gophish v0.12.1,
originally created by Jordan Wright.

----------------------------------------------------------------
Intended Use Notice (Non-Binding Advisory)
----------------------------------------------------------------

Anglerphish is intended exclusively for authorized security
testing, phishing simulations, user awareness training,
and defensive cybersecurity research.

Users are responsible for ensuring compliance with all
applicable laws and obtaining proper authorization before use.

This notice does not modify or supersede the terms of the MIT License.
```
