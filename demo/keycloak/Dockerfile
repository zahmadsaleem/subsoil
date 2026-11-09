FROM maven:3.9.11-eclipse-temurin-21 AS mapper
WORKDIR /build
COPY pom.xml .
COPY src src
RUN --mount=type=cache,target=/root/.m2 mvn -q -DskipTests package
FROM quay.io/keycloak/keycloak:26.7.0
COPY --from=mapper /build/target/registry-token-mapper-1.0.0.jar /opt/keycloak/providers/registry-token-mapper.jar
