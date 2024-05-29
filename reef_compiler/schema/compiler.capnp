interface CompilerManager {
    struct Config {
        mainPath @0 : Text;
        depPath @1 : Text;
    }

    new @2 (mainPath :Text, skeletonPath :Text) -> (manager :CompilerManager);

    compile @3 (programSrc :Text, language :Language) -> (response :CompilerResponse);
}

union CompilerResponse {
    fileContent @0 :Data;
    compilerError @1 :Text;
    fileSystemError @2 :Text;
}

enum Language {
    c @0;
    rust @1;
}
