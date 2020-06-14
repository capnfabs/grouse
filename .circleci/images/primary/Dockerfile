FROM circleci/golang:1.13.6-buster

ENV HUGO_RELEASE https://github.com/gohugoio/hugo/releases/download/v0.62.2/hugo_0.62.2_Linux-64bit.deb
RUN curl -L ${HUGO_RELEASE} -o hugo.deb && sudo dpkg -i hugo.deb && rm hugo.deb
