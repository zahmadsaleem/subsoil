package dev.subsoil.demo;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.keycloak.models.ClientSessionContext;
import org.keycloak.models.KeycloakSession;
import org.keycloak.models.ProtocolMapperModel;
import org.keycloak.models.RoleModel;
import org.keycloak.models.UserSessionModel;
import org.keycloak.protocol.oidc.mappers.AbstractOIDCProtocolMapper;
import org.keycloak.protocol.oidc.mappers.OIDCAccessTokenMapper;
import org.keycloak.provider.ProviderConfigProperty;
import org.keycloak.representations.AccessToken;
import org.keycloak.services.ErrorResponseException;

/** Demo adapter: OAuth scopes intersect realm roles, encoded as Distribution access claims. */
public final class RegistryTokenMapper extends AbstractOIDCProtocolMapper implements OIDCAccessTokenMapper {
    public static final String ID = "subsoil-demo-registry-token";
    @Override public String getId() { return ID; }
    @Override public String getDisplayType() { return "OCI registry repository permissions"; }
    @Override public String getDisplayCategory() { return "Token mapper"; }
    @Override public String getHelpText() { return "Grants only requested repository actions backed by identically named realm roles."; }
    @Override public int getPriority() { return 50; }
    @Override public List<ProviderConfigProperty> getConfigProperties() { return List.of(); }
    @Override public AccessToken transformAccessToken(AccessToken token, ProtocolMapperModel mapping, KeycloakSession session, UserSessionModel user, ClientSessionContext client) {
        String resource = session.getContext().getHttpRequest().getDecodedFormParameters().getFirst("resource");
        String expected = mapping.getConfig().get("registry");
        if (resource != null && !resource.equals(expected)) {
            throw new ErrorResponseException("invalid_target", "Choose the registry configured for this client", jakarta.ws.rs.core.Response.Status.BAD_REQUEST);
        }
        String scopes = client.getScopeString();
        boolean workspaceBrowser = "true".equals(mapping.getConfig().get("workspaceBrowser"));
        String workspaceBinding = mapping.getConfig().getOrDefault("workspaceRepository", "");
        String workspace = WorkspaceAccess.repository(user.getUser());
        boolean workspaceAllowed = !workspace.isEmpty() && (workspaceBrowser || workspace.equals(workspaceBinding));
        Map<String, Set<String>> repositories = new LinkedHashMap<>();
        Set<String> granted = new LinkedHashSet<>();
        boolean catalogRequested = false;
        for (String scope : scopes.split(" ")) {
            if (scope.equals("workspace:read") || scope.equals("workspace:write")) {
                if (workspaceAllowed) {
                    repositories.computeIfAbsent(workspace, key -> new LinkedHashSet<>())
                            .add(scope.equals("workspace:read") ? "pull" : "push");
                    granted.add(scope);
                    catalogRequested |= workspaceBrowser && scope.equals("workspace:read");
                }
                continue;
            }
            if (scope.startsWith("repository:") && !workspaceBinding.isEmpty()) continue;
            if (scope.equals("registry:catalog:read")) { catalogRequested = true; continue; }
            if (!scope.startsWith("repository:")) { if (!scope.isBlank()) granted.add(scope); continue; }
            String[] parts = scope.split(":", -1);
            if (parts.length != 3 || !parts[1].matches("[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*")) continue;
            Set<String> actions = repositories.computeIfAbsent(parts[1], k -> new LinkedHashSet<>());
            for (String action : parts[2].split(",")) {
                if (!action.equals("pull") && !action.equals("push")) continue;
                RoleModel role = session.getContext().getRealm().getRole("repository:" + parts[1] + ":" + action);
                if (role != null && user.getUser().hasRole(role)) actions.add(action);
            }
        }
        List<Map<String, Object>> access = new ArrayList<>();
        boolean canRead = false;
        for (var repository : repositories.entrySet()) {
            if (repository.getValue().isEmpty()) continue;
            access.add(Map.of("type", "repository", "name", repository.getKey(), "actions", new ArrayList<>(repository.getValue())));
            granted.add("repository:" + repository.getKey() + ":" + String.join(",", repository.getValue()));
            canRead |= repository.getValue().contains("pull");
        }
        if (catalogRequested && canRead && workspaceBinding.isEmpty()) {
            access.add(Map.of("type", "registry", "name", "catalog", "actions", List.of("*")));
            // Distribution catalog implementations use both of these established scope forms.
            access.add(Map.of("type", "repository", "name", "", "actions", List.of("pull")));
            granted.add("registry:catalog:read");
        }
        token.getOtherClaims().put("access", access);
        token.addAudience(mapping.getConfig().getOrDefault("service", "zot"));
        if (workspaceAllowed && workspaceBrowser) {
            token.addAudience("subsoil-registrar");
        }
        String result = String.join(" ", granted);
        token.setScope(result);
        return token;
    }
}
