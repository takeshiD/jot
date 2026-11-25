{
  description = "Jot — a Today-centric keyboard TUI for Taskwarrior";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
      version = "0.1.0";
    in
    {
      packages = forAllSystems (pkgs: {
        jot = pkgs.buildGoModule {
          pname = "jot";
          inherit version;
          src = self;

          # Dependencies are vendored in ./vendor, so no vendorHash is needed.
          vendorHash = null;
          subPackages = [ "cmd/jot" ];

          env.CGO_ENABLED = 0;
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];

          # The integration tests drive a real `task` binary against an
          # isolated temporary TASKRC/TASKDATA.
          nativeCheckInputs = [ pkgs.taskwarrior3 ];
          # subPackages would limit tests to cmd/jot; run the whole module.
          checkPhase = ''
            runHook preCheck
            export HOME=$(mktemp -d)
            go test -mod=vendor -count=1 ./...
            runHook postCheck
          '';

          meta = {
            description = "Today-centric keyboard TUI for Taskwarrior";
            homepage = "https://github.com/takeshiD/jot";
            mainProgram = "jot";
            platforms = pkgs.lib.platforms.unix;
          };
        };
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.jot;
      });

      apps = forAllSystems (pkgs: {
        default = {
          type = "app";
          meta.description = "Run Jot";
          program = nixpkgs.lib.getExe self.packages.${pkgs.stdenv.hostPlatform.system}.jot;
        };
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.gopls
            pkgs.gnumake
            pkgs.taskwarrior3
          ];
          env.CGO_ENABLED = "0";
        };
      });

      checks = forAllSystems (pkgs: {
        jot = self.packages.${pkgs.stdenv.hostPlatform.system}.jot;
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
