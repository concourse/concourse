module Main exposing (main)

import Effects
import Layout
import Msgs
import Navigation
import Subscription


main : Program Layout.Flags Layout.Model Msgs.Msg
main =
    Navigation.programWithFlags Layout.locationMsg
        { init = \flags -> Layout.init flags >> Tuple.mapSecond effectsToCmd
        , update = \msg -> Layout.update msg >> Tuple.mapSecond effectsToCmd
        , view = Layout.view
        , subscriptions = Layout.subscriptions >> subscriptionsToSub
        }


effectsToCmd : List ( Effects.LayoutDispatch, Effects.Effect ) -> Cmd Msgs.Msg
effectsToCmd =
    List.map effectToCmd >> Cmd.batch


effectToCmd : ( Effects.LayoutDispatch, Effects.Effect ) -> Cmd Msgs.Msg
effectToCmd ( disp, eff ) =
    Effects.runEffect eff |> Cmd.map (Msgs.Callback disp)


subscriptionsToSub : List (Subscription.Subscription Msgs.Msg) -> Sub Msgs.Msg
subscriptionsToSub =
    List.map Subscription.runSubscription >> Sub.batch
