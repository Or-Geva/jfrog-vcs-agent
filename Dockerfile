FROM releases-docker.jfrog.io/jfrog-ecosystem-integration-env:1.2.0
RUN curl -fL https://getcli.jfrog.io | sh && mv ./jfrog /usr/local/bin/
ENV ORIGIN_PATH="${PATH}" PATH="/usr/local/jfrog/bin:${PATH}"
WORKDIR /agent_home/src
COPY . /agent_home/src
COPY ./script /usr/local/jfrog/bin/
RUN go build -o executor && mv ./executor /usr/local/bin/
WORKDIR /agent_home/workspace
CMD ["executor"]