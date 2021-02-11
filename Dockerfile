FROM releases-docker.jfrog.io/jfrog-ecosystem-integration-env:1.2.0
RUN curl -fL https://getcli.jfrog.io | sh &&  mv ./jfrog /usr/local/bin/
RUN mkdir -p /usr/local/jfrog/bin && \
    echo -ne '#!/bin/bash\nexport PATH=${ORIGIN_PATH}\nfor el in "$@"; do case "$el" in i|install) cmd="npmi" ;; ci) cmd="npmci" ;; *) args+="$el " ;;  esac; done; if [[ $cmd ]]; then command jfrog rt $cmd $args; else command /usr/bin/npm $@; fi;' >  /usr/local/jfrog/bin/npm && chmod +x /usr/local/jfrog/bin/npm && \
    echo -ne '#!/bin/bash\nexport PATH=${ORIGIN_PATH}\ncommand jfrog rt mvn $@' > /usr/local/jfrog/bin/mvn && chmod +x /usr/local/jfrog/bin/mvn  && \
    echo -ne '#!/bin/bash\nexport PATH=${ORIGIN_PATH}\ncommand jfrog rt gradle $@' > /usr/local/jfrog/bin/gradle && chmod +x /usr/local/jfrog/bin/gradle
ENV ORIGIN_PATH="${PATH}" PATH="/usr/local/jfrog/bin:${PATH}"
WORKDIR /workspace
COPY . /workspace
RUN  go build -o executor
CMD ["/workspace/executor"]