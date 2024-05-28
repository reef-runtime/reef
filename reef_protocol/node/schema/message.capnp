using Go = import "/go.capnp";

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
        empty               @1 :Void;

        assignID            @2 :AssignIDMessage;

        startJob            @3 :JobInitializationMessage;
        abortJob            @4 :JobKillMessage;
    }
}

struct AssignIDMessage {
    nodeID @0: Data;
}

struct JobInitializationMessage {
    workerIndex @0 :UInt32;
    jobID @1 :Text;
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

    jobLog              @2;
    jobProgressReport   @3;
}

struct MessageFromNode {
    kind @0 :MessageFromNodeKind;

    body :union {
        empty                   @1 :Void;
        jobLog                  @2 :JobLogMessage;
        jobProgressReport       @3 :JobProgressReportMessage;
    }
}

struct HandshakeRespondMessage {
    numWorkers @0 :UInt16;
    nodeName @1 :Text;
}


struct JobStartedMessage {
    workerIndex @0 :UInt32;
    jobID @1 :Text;
}

struct JobLogMessage {
    logKind @0 :UInt16;
    workerIndex @1 :UInt16;
    content @2 :Data;
}

struct JobProgressReportMessage {
    workerIndex @0 :UInt16;
    # Maps from 0..=100 onto the full int range.
    progress @1 :UInt16;
}
