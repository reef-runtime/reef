import { throws } from "assert"

export type TopicKind = 'nodes' | 'single_job' | 'all_jobs'

export interface Topic {
    kind: TopicKind
    additional?: string
}

export type OnMessageCallBack<T> = (data: T) => void

export function openWS<T>(topics: Set<Topic>, onMessage: OnMessageCallBack<T>) {
    let protocol = undefined
    switch (document.location.protocol) {
        case 'http:':
            protocol = 'ws:'
            break
        case 'https:':
            protocol = 'wss:'
            break
        default:
            throw `Unsupported protocol '${document.location.protocol}':
                    only http and https are supported`
    }

    // Build the websocket URL from the components.
    let url = `${protocol}//${location.host}/api/updates`

    let conn = new WebSocket(url)

    conn.onopen = () => {
        // Subscribe to wanted topics.
        let topicsList: Topic[] = Array.from(topics.values())
        conn.send(JSON.stringify(topicsList))
    }

    conn.onclose = () => {
        throw "Websocket closed prematurely"
    }

    conn.onmessage = (evt) => {
        let payload = JSON.parse(evt.data)
        onMessage(payload)
    }
}
