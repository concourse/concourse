module Main exposing (main)

import Application.Application as Application
import Browser
import Browser.Navigation as Navigation
import Concourse
import Message.Effects as Effects
import Message.Subscription as Subscription
import Message.TopLevelMessage as Msgs
import Url


type alias TopLevelModel =
    { key : Navigation.Key
    , model : Application.Model
    }


init :
    Application.Flags
    -> Url.Url
    -> Navigation.Key
    -> ( TopLevelModel, Cmd Msgs.TopLevelMessage )
init flags url key =
    let
        ( model, effects ) =
            Application.init flags url
    in
    ( { key = key, model = model }, effects )
        |> effectsToCmd


update :
    Msgs.TopLevelMessage
    -> TopLevelModel
    -> ( TopLevelModel, Cmd Msgs.TopLevelMessage )
update msg model =
    let
        ( appModel, effects ) =
            Application.update msg model.model
    in
    ( { model | model = appModel }, effects )
        |> effectsToCmd


main : Program Application.Flags TopLevelModel Msgs.TopLevelMessage
main =
    Browser.application
        { init = init
        , update = update
        , view = view
        , subscriptions =
            .model
                >> Application.subscriptions
                >> subscriptionsToSub
        , onUrlChange = Application.locationMsg
        , onUrlRequest = Subscription.UrlRequest >> Msgs.DeliveryReceived
        }


view : TopLevelModel -> Browser.Document Msgs.TopLevelMessage
view model =
    Application.view model.model


effectsToCmd :
    ( TopLevelModel, List Effects.Effect )
    -> ( TopLevelModel, Cmd Msgs.TopLevelMessage )
effectsToCmd ( model, effs ) =
    ( model
    , List.map (effectToCmd model.model.session.csrfToken model.key) effs |> Cmd.batch
    )



-- there's a case to be made that this function should actually
-- accept a Session


effectToCmd :
    Concourse.CSRFToken
    -> Navigation.Key
    -> Effects.Effect
    -> Cmd Msgs.TopLevelMessage
effectToCmd csrfToken key eff =
    Effects.runEffect eff key csrfToken |> Cmd.map Msgs.Callback


subscriptionsToSub : List Subscription.Subscription -> Sub Msgs.TopLevelMessage
subscriptionsToSub =
    List.map Subscription.runSubscription
        >> Sub.batch
        >> Sub.map Msgs.DeliveryReceived
