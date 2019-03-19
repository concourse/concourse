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
        { init = \flags -> Application.init flags >> effectsToCmd
        , update = \msg -> Application.update msg >> effectsToCmd
        , view = Application.view
        , subscriptions = Application.subscriptions >> subscriptionsToSub
        }


effectsToCmd : ( Application.Model, List Effects.Effect ) -> ( Application.Model, Cmd Msgs.Msg )
effectsToCmd ( model, effs ) =
    ( model, List.map (effectToCmd model.csrfToken) effs |> Cmd.batch )


effectToCmd : Concourse.CSRFToken -> Effects.Effect -> Cmd Msgs.Msg
effectToCmd csrfToken eff =
    Effects.runEffect eff csrfToken |> Cmd.map Msgs.Callback


subscriptionsToSub : List Subscription.Subscription -> Sub Msgs.Msg
subscriptionsToSub =
    List.map Subscription.runSubscription >> Sub.batch >> Sub.map Msgs.DeliveryReceived
