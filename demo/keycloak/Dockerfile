FROM maven:3.9.11-eclipse-temurin-21 AS mapper
WORKDIR /build
COPY demo/keycloak/pom.xml .
COPY demo/keycloak/src src
RUN --mount=type=cache,target=/root/.m2 mvn -q -DskipTests package
FROM quay.io/keycloak/keycloak:26.7.0
COPY --from=mapper /build/target/registry-token-mapper-1.0.0.jar /opt/keycloak/providers/registry-token-mapper.jar
COPY demo/keycloak/theme/subsoil /opt/keycloak/themes/subsoil
COPY assets/brand/subsoil-logo.svg assets/brand/subsoil-symbol.svg /opt/keycloak/themes/subsoil/login/resources/img/
COPY assets/brand/subsoil-logo.svg assets/brand/subsoil-symbol.svg /opt/keycloak/themes/subsoil/account/resources/img/
COPY demo/keycloak/theme/subsoil.css /opt/keycloak/themes/subsoil/login/resources/css/subsoil.css
COPY demo/keycloak/theme/subsoil.css /opt/keycloak/themes/subsoil/account/resources/css/subsoil.css
