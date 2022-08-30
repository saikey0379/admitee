FROM docker.v2.aispeech.com/aispeech/centos:7.6.1810
ENV TZ=Asia/Shanghai
ADD admiteed /admiteed
CMD /admiteed
