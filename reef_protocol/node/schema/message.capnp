using Go = import "./go.capnp";

@0xc5f4c7dc14cbdbf0;

$Go.package("message");
$Go.import("foo/message");

#
# Messages sent from the manager to the node.
#

enum MessageToNodeKind {
    ping            @0;
    pong            @1;

    assignID        @2;
    initHandShake   @3;
    startJob        @4;
}

struct MessageToNode {
    kind @0 :MessageToNodeKind;

    body :union {
        empty               @1  :Void;
        assignID            @2  :AssignIDMessage;
        startJob            @3  :JobInitializationMessage;
        resumeJob           @4  :JobResumeMessage;
        abortJob            @5  :JobKillMessage;
    }
}

struct AssignIDMessage {
    nodeID @0: Data;
}

struct JobResumeMessage {
    job             @0 :JobInitializationMessage;
    previousState   @1: PreviousJobState;
}


struct PreviousJobState {
    progress            @0 :Float32;
    interpreterState    @1 :Data;
}

struct JobInitializationMessage {
    workerIndex     @0 :UInt32;
    jobID           @1 :Text;
    programByteCode @2 :Data;
}

struct JobKillMessage {
    jobID @0 :Text;
}


#
# Messages sent by the node, to be received by the manager.
#

enum MessageFromNodeKind {
    ping                @0;
    pong                @1;

    jobStateSync        @2;
}

struct MessageFromNode {
    kind @0 :MessageFromNodeKind;

    body :union {
        empty           @1 :Void;
        jobStateSync    @2 :JobStateSync;
    }
}

struct HandshakeRespondMessage {
    numWorkers  @0 :UInt16;
    nodeName    @1 :Text;
}

struct JobStartedMessage {
    workerIndex @0 :UInt32;
    jobID       @1 :Text;
}

struct JobLogMessage {
    logKind     @0 :UInt16;
    content     @1 :Data;
}

struct JobStateSync {
    workerIndex @0 :UInt16;
    # Maps progress from 0..=1.0
    progress    @1 :Float32;
    logs        @2 :List(JobLogMessage);
    interpreter @3 :Data;
}
