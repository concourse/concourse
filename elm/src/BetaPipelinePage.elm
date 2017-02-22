port module BetaPipelinePage exposing (main)

import BetaPipeline


port setGroups : (List String -> msg) -> Sub msg


main : Program BetaPipeline.Flags
main =
    Html.programWithFlags
        { init = BetaPipeline.init { setGroups = setGroups }
        , update = BetaPipeline.update
        , view = BetaPipeline.view
        , subscriptions = BetaPipeline.subscriptions
        }
