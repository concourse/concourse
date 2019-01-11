module FlySuccess.Effects exposing (Effect(..), runEffect)

import FlySuccess.Msgs exposing (Msg(..))
import Http
import QueryString


type Effect
    = SendTokenToFly String Int


runEffect : Effect -> Cmd Msg
runEffect effect =
    case effect of
        SendTokenToFly authToken flyPort ->
            sendTokenToFly authToken flyPort


sendTokenToFly : String -> Int -> Cmd Msg
sendTokenToFly authToken flyPort =
    let
        queryString =
            QueryString.empty
                |> QueryString.add "token" authToken
                |> QueryString.render
    in
    Http.request
        { method = "GET"
        , headers = []
        , url = "http://127.0.0.1:" ++ toString flyPort ++ queryString
        , body = Http.emptyBody
        , expect = Http.expectStringResponse (\_ -> Ok ())
        , timeout = Nothing
        , withCredentials = False
        }
        |> Http.send (\r -> TokenSentToFly (r == Ok ()))
