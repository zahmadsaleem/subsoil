package dev.subsoil.demo;

import java.util.ArrayList;
import java.util.List;
import org.keycloak.models.AuthenticatedClientSessionModel;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.ProtocolMapperModel;
import org.keycloak.models.RoleModel;
import org.keycloak.models.UserSessionModel;
import org.keycloak.protocol.docker.DockerAuthV2Protocol;
import org.keycloak.protocol.docker.mapper.DockerAuthV2ProtocolMapper;
import org.keycloak.protocol.docker.mapper.DockerAuthV2AttributeMapper;
import org.keycloak.representations.docker.DockerAccess;
import org.keycloak.representations.docker.DockerResponseToken;

/** Keeps ordinary OCI command-line clients on the same role policy as browser PKCE. */
public final class RegistryDockerMapper extends DockerAuthV2ProtocolMapper implements DockerAuthV2AttributeMapper {
    @Override public String getId() { return "subsoil-demo-docker-permissions"; }
    @Override public String getDisplayType() { return "OCI repository role permissions"; }
    @Override public String getHelpText() { return "Intersects requested pull/push actions with identically named realm roles."; }
    @Override public boolean appliesTo(DockerResponseToken token) { return true; }
    @Override public DockerResponseToken transformDockerResponseToken(DockerResponseToken token, ProtocolMapperModel mapping, KeycloakSession session, UserSessionModel user, AuthenticatedClientSessionModel client) {
        token.getAccessItems().clear();
        String scopes = client.getNote(DockerAuthV2Protocol.SCOPE_PARAM);
        if (scopes == null) return token;
        for (String scope : scopes.split(" ")) {
            String[] parts = scope.split(":", -1);
            if (parts.length != 3 || !parts[0].equals("repository") || !parts[1].matches("[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*")) continue;
            List<String> actions = new ArrayList<>();
            for (String action : parts[2].split(",")) {
                if (!action.equals("pull") && !action.equals("push")) continue;
                RoleModel role = session.getContext().getRealm().getRole("repository:"+parts[1]+":"+action);
                if (role != null && user.getUser().hasRole(role)) actions.add(action);
            }
            if (!actions.isEmpty()) token.getAccessItems().add(new DockerAccess("repository:"+parts[1]+":"+String.join(",",actions)));
        }
        return token;
    }
}
