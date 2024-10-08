using Go = import "./go.capnp";

@0xc5f4c7dc14cbdbf0;

$Go.package("message");
$Go.import("foo/message");

#
# Messages sent from the manager to the node.
#

enum MessageToNodeKind {
    ping                @0;

    initHandShake       @1;
    assignId            @2;
    startJob            @3;
    abortJob            @4;
}

struct MessageToNode {
    kind @0 :MessageToNodeKind;

    body :union {
        empty           @1  :Void;
        assignId        @2  :AssignIdMessage;
        startJob        @3  :JobStartMessage;
        abortJob        @4  :JobAbortMessage;
    }
}

struct AssignIdMessage {
    nodeId @0: Data;
}

struct JobStartMessage {
    workerIndex         @0 :UInt32;
    jobId               @1 :Text;
    programByteCode     @2 :Data;
    datasetId           @3 :Text;

    # If the job has just been started these will be 0/empty.
    progress            @4 :Float32;
    interpreterState    @5 :Data;
}

struct JobAbortMessage {
    jobId @0 :Text;
}


#
# Messages sent by the node, to be received by the manager.
#

enum MessageFromNodeKind {
    handshakeResponse   @0;

    jobStateSync        @1;
    jobResult           @2;
}

struct MessageFromNode {
    kind @0 :MessageFromNodeKind;

    body :union {
        handshakeResponse   @1 :HandshakeRespondMessage;
        jobStateSync        @2 :JobStateSync;
        jobResult           @3 :JobResult;
    }
}

struct HandshakeRespondMessage {
    numWorkers          @0 :UInt16;
    nodeName            @1 :Text;
}

struct JobStateSync {
    workerIndex         @0 :UInt16;
    # Maps progress from 0..=1.0
    progress            @1 :Float32;
    logs                @2 :List(JobLogMessage);
    interpreter         @3 :Data;
}

struct JobLogMessage {
    logKind             @0 :UInt16;
    content             @1 :Data;
}

struct JobResult {
    workerIndex         @0: UInt16;
    success             @1: Bool;
    contentType         @2: ResultContentType;
    contents            @3: Data;
}

enum ResultContentType {
	i32                 @0;
	bytes               @1;
	stringPlain         @2;
	stringJSON          @3;
}
