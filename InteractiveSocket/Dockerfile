FROM ubuntu

RUN apt-get update && \
    apt-get install -y \
    git \
    golang-1.10 \
    vim
WORKDIR /usr/lib/go-1.10/src/github.com/methanduck/GO/
RUN git init && \
    git config core.sparseCheckout true && \
    git remote add -f origin https://github.com/methanduck/GO && \
    echo "InteractiveSocket/" >> .git/info/sparse-checkout && \
    git pull origin master  && \
    ln -s /usr/lib/go-1.10/bin/go /usr/local/bin/go
ENV GOPATH /usr/lib/go-1.10/src/
COPY RunWindow.sh /usr/lib/go-1.10/src/github.com/methanduck/GO
ENTRYPOINT [ "./RunWindow.sh" ]