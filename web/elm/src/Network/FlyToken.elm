module Network.FlyToken exposing (sendTokenToFly)

import Http
import Routes
import Task


sendTokenToFly : String -> Int -> Task.Task Http.Error ()
sendTokenToFly authToken flyPort =
    Http.toTask <|
        Http.request
            { method = "GET"
            , headers = []
            , url = Routes.tokenToFlyRoute authToken flyPort
            , body = Http.emptyBody
            , expect = Http.expectStringResponse (\_ -> Ok ())
            , timeout = Nothing
            , withCredentials = False
            }
