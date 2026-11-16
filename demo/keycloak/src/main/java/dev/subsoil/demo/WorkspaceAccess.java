package dev.subsoil.demo;

import org.keycloak.models.UserModel;

final class WorkspaceAccess {
    private WorkspaceAccess() {}

    static String repository(UserModel user) {
        String id = user.getId();
        if (!user.isEmailVerified() || user.getEmail() == null
                || user.getEmail().endsWith("@fixtures.subsoil.invalid")
                || !id.matches("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")) {
            return "";
        }
        return "workspaces/" + id + "/scratch";
    }
}
