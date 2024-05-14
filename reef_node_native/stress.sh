perl -e 'print "--manager-url=https://localhost:3000\n" x 10' | xargs -P 10 -I {} ../target/debug/reef_node_native {}
