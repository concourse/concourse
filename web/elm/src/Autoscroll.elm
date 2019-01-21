module Autoscroll exposing
    ( Model
    , Msg(SubMsg)
    , ScrollBehavior(..)
    , subscriptions
    , update
    , view
    )

import AnimationFrame
import Effects
import Html exposing (Html)


type alias Model subModel =
    { scrollBehaviorFunc : subModel -> ScrollBehavior
    , subModel : subModel
    }


type ScrollBehavior
    = ScrollElement String
    | ScrollWindow
    | NoScroll


type Msg subMsg
    = SubMsg subMsg
    | ScrollDown


update :
    (subMsg -> subModel -> ( subModel, List Effects.Effect ))
    -> Msg subMsg
    -> Model subModel
    -> ( Model subModel, List Effects.Effect )
update subUpdate action model =
    case action of
        SubMsg subMsg ->
            let
                ( subModel, subCmd ) =
                    subUpdate subMsg model.subModel
            in
            ( { model | subModel = subModel }, subCmd )

        ScrollDown ->
            ( model
            , case model.scrollBehaviorFunc model.subModel of
                ScrollElement ele ->
                    [ Effects.Scroll (Effects.ToBottomOf ele) ]

                ScrollWindow ->
                    [ Effects.Scroll Effects.ToWindowBottom ]

                NoScroll ->
                    []
            )


view : (subModel -> Html subMsg) -> Model subModel -> Html (Msg subMsg)
view subView model =
    Html.map SubMsg (subView model.subModel)


subscriptions : (subModel -> Sub subMsg) -> Model subModel -> Sub (Msg subMsg)
subscriptions subSubscriptions model =
    let
        subSubs =
            Sub.map SubMsg (subSubscriptions model.subModel)
    in
    if model.scrollBehaviorFunc model.subModel /= NoScroll then
        Sub.batch
            [ AnimationFrame.times (always ScrollDown)
            , subSubs
            ]

    else
        subSubs
