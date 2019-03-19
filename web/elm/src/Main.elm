module Main exposing (main)

import Application.Application as Application
import Concourse
import Message.Effects as Effects
import Message.Subscription as Subscription
import Message.TopLevelMessage as Msgs
import Navigation


main : Program Application.Flags Application.Model Msgs.TopLevelMessage
main =
    Navigation.programWithFlags Application.locationMsg
        { init = \flags -> Application.init flags >> effectsToCmd
        , update = \msg -> Application.update msg >> effectsToCmd
        , view = Application.view
        , subscriptions = Application.subscriptions >> subscriptionsToSub
        }


effectsToCmd : ( Application.Model, List Effects.Effect ) -> ( Application.Model, Cmd Msgs.TopLevelMessage )
effectsToCmd ( model, effs ) =
    ( model, List.map (effectToCmd model.csrfToken) effs |> Cmd.batch )


effectToCmd : Concourse.CSRFToken -> Effects.Effect -> Cmd Msgs.TopLevelMessage
effectToCmd csrfToken eff =
    Effects.runEffect eff csrfToken |> Cmd.map Msgs.Callback


subscriptionsToSub : List Subscription.Subscription -> Sub Msgs.TopLevelMessage
subscriptionsToSub =
    List.map Subscription.runSubscription >> Sub.batch >> Sub.map Msgs.DeliveryReceived
