#!/bin/sh

if [ -z "$1" ]
  then
    echo "No Gateway URL supplied"
fi

GATEWAY_HOST=$1

DATE=$(date "+%Y-%m-%d")

curl -X POST "${GATEWAY_HOST}/api/events" -H "accept: application/octet-stream" -H "Content-Type: application/json" -d "{\"name\":\"Lifetime Tour 1\",\"address\":\"Cmo. de Perales, 23, 28041\",\"city\":\"MADRID\",\"province\":\"MADRID\",\"country\":\"SPAIN\",\"date\":\"${DATE}\",\"startTime\":\"18:00\",\"endTime\":\"23:00\",\"location\":\"Caja Magica\",\"artist\":\"Guns n Roses\",\"description\":\"The revived Guns N’ Roses and ...\",\"image\":\"guns-P1080795.jpg\"}"
curl -X POST "${GATEWAY_HOST}/api/events" -H "accept: application/octet-stream" -H "Content-Type: application/json" -d "{\"name\":\"Lifetime Tour 2\",\"address\":\"Cmo. de Perales, 23, 28041\",\"city\":\"MADRID\",\"province\":\"MADRID\",\"country\":\"SPAIN\",\"date\":\"${DATE}\",\"startTime\":\"18:00\",\"endTime\":\"23:00\",\"location\":\"Caja Magica\",\"artist\":\"Guns n Roses\",\"description\":\"The revived Guns N’ Roses and ...\",\"image\":\"guns-P1080795.jpg\"}"
curl -X POST "${GATEWAY_HOST}/api/events" -H "accept: application/octet-stream" -H "Content-Type: application/json" -d "{\"name\":\"Lifetime Tour 3\",\"address\":\"Cmo. de Perales, 23, 28041\",\"city\":\"MADRID\",\"province\":\"MADRID\",\"country\":\"SPAIN\",\"date\":\"${DATE}\",\"startTime\":\"18:00\",\"endTime\":\"23:00\",\"location\":\"Caja Magica\",\"artist\":\"Guns n Roses\",\"description\":\"The revived Guns N’ Roses and ...\",\"image\":\"guns-P1080795.jpg\"}"
curl -X POST "${GATEWAY_HOST}/api/events" -H "accept: application/octet-stream" -H "Content-Type: application/json" -d "{\"name\":\"Lifetime Tour 4\",\"address\":\"Cmo. de Perales, 23, 28041\",\"city\":\"MADRID\",\"province\":\"MADRID\",\"country\":\"SPAIN\",\"date\":\"${DATE}\",\"startTime\":\"18:00\",\"endTime\":\"23:00\",\"location\":\"Caja Magica\",\"artist\":\"Guns n Roses\",\"description\":\"The revived Guns N’ Roses and ...\",\"image\":\"guns-P1080795.jpg\"}"
curl -X POST "${GATEWAY_HOST}/api/events" -H "accept: application/octet-stream" -H "Content-Type: application/json" -d "{\"name\":\"Lifetime Tour 5\",\"address\":\"Cmo. de Perales, 23, 28041\",\"city\":\"MADRID\",\"province\":\"MADRID\",\"country\":\"SPAIN\",\"date\":\"${DATE}\",\"startTime\":\"18:00\",\"endTime\":\"23:00\",\"location\":\"Caja Magica\",\"artist\":\"Guns n Roses\",\"description\":\"The revived Guns N’ Roses and ...\",\"image\":\"guns-P1080795.jpg\"}"

