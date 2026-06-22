FROM python:3.11-slim

WORKDIR /app

RUN pip install --no-cache-dir paho-mqtt

COPY sensor_simulator.py .

ENV UDP_HOST=backend
ENV UDP_PORT=8001
ENV WIND_SPEED=
ENV WIND_DIRECTION=
ENV MQTT_BROKER=
ENV FAST_MODE=true

CMD ["python", "sensor_simulator.py", "--fast", "--host", "backend", "--port", "8001"]
