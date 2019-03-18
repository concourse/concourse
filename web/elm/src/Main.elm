module Main exposing (main)

import Application.Application as Application
import Concourse
import Message.ApplicationMsgs as Msgs
import Message.Effects as Effects
import Message.Subscription as Subscription
import Navigation


main : Program Application.Flags Application.Model Msgs.Msg
main =
    Navigation.programWithFlags Application.locationMsg
        { init = \flags -> Application.init flags >> Tuple.mapSecond effectsToCmd
        , update = \msg -> Application.update msg >> Tuple.mapSecond effectsToCmd
        , view = Application.view
        , subscriptions = Application.subscriptions >> subscriptionsToSub
        }


effectsToCmd : List ( Effects.LayoutDispatch, Concourse.CSRFToken, Effects.Effect ) -> Cmd Msgs.Msg
effectsToCmd =
    List.map effectToCmd >> Cmd.batch


effectToCmd : ( Effects.LayoutDispatch, Concourse.CSRFToken, Effects.Effect ) -> Cmd Msgs.Msg
effectToCmd ( disp, csrfToken, eff ) =
    Effects.runEffect eff csrfToken |> Cmd.map (Msgs.Callback disp)


subscriptionsToSub : List Subscription.Subscription -> Sub Msgs.Msg
subscriptionsToSub =
    List.map Subscription.runSubscription >> Sub.batch >> Sub.map Msgs.DeliveryReceived
