with import <nixpkgs> {};

pkgs.mkShell {
  buildInputs = [
    stdenv
    glibc.static
    gcc
    go
    gocode
    gopls
    libvterm
    delve
  ];
  shellHook =''
    export GOBIN=$(pwd)/bin
    export GOPATH=$HOME/.go
  '';
  CFLAGS="-I${pkgs.glibc.dev}/include";
  LDFLAGS="-L${pkgs.glibc}/lib";
}

