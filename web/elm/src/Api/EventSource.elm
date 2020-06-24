module Api.EventSource exposing (Event(..), EventEnvelope, decodeEventEnvelope)

import Json.Decode


type alias EventEnvelope a =
    { data : Event a
    , url : String
    }


type Event a
    = Opened
    | NetworkError
    | Event a


decodeEventEnvelope :
    (String -> Json.Decode.Decoder a)
    -> Json.Decode.Decoder (EventEnvelope a)
decodeEventEnvelope eventDecoder =
    let
        dataDecoder =
            Json.Decode.field "type" Json.Decode.string
                |> Json.Decode.andThen
                    (\t ->
                        case t of
                            "open" ->
                                Json.Decode.succeed Opened

                            "error" ->
                                Json.Decode.succeed NetworkError

                            v ->
                                eventDecoder v |> Json.Decode.map Event
                    )

        urlDecoder =
            Json.Decode.at [ "target", "url" ] Json.Decode.string
    in
    Json.Decode.map2 EventEnvelope
        dataDecoder
        urlDecoder
