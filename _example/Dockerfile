FROM perl:5.26

RUN useradd app && mkdir -p /home/app && chown -R app /home/app

RUN cpanm --notest Carton App::cpm

COPY ./cpanfile /opt/webapp/cpanfile
WORKDIR /opt/webapp

RUN cpm install
RUN carton install

COPY ./ /opt/webapp

RUN chown -R app /opt/webapp

USER app

EXPOSE 12346

CMD carton exec -- plackup -s Starlet --port=12346 --max-workers=4 app.psgi 
