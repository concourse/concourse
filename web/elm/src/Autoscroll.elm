module Autoscroll
    exposing
        ( init
        , update
        , urlUpdate
        , view
        , subscriptions
        , ScrollBehavior(..)
        , Msg(SubMsg)
        , Model
        )

import AnimationFrame
import Html exposing (Html)
import Task
import Scroll
import UpdateMsg exposing (UpdateMsg)


type alias Model subModel =
    { subModel : subModel
    , scrollBehaviorFunc : subModel -> ScrollBehavior
    }


type ScrollBehavior
    = ScrollElement String
    | ScrollWindow
    | NoScroll


type Msg subMsg
    = SubMsg subMsg
    | ScrollDown
    | ScrolledDown


init : (subModel -> ScrollBehavior) -> ( subModel, Cmd subMsg ) -> ( Model subModel, Cmd (Msg subMsg) )
init toScrollMsg ( subModel, subCmd ) =
    ( Model subModel toScrollMsg, Cmd.map SubMsg subCmd )


update : (subMsg -> subModel -> ( subModel, Cmd subMsg, Maybe UpdateMsg )) -> Msg subMsg -> Model subModel -> ( Model subModel, Cmd (Msg subMsg), Maybe UpdateMsg )
update subUpdate action model =
    case action of
        SubMsg subMsg ->
            let
                ( subModel, subCmd, subUpdateMsg ) =
                    subUpdate subMsg model.subModel
            in
                ( { model | subModel = subModel }, Cmd.map SubMsg subCmd, subUpdateMsg )

        ScrollDown ->
            ( model
            , case model.scrollBehaviorFunc model.subModel of
                ScrollElement ele ->
                    scrollToBottom ele

                ScrollWindow ->
                    scrollToWindowBottom

                NoScroll ->
                    Cmd.none
            , Nothing
            )

        ScrolledDown ->
            ( model, Cmd.none, Nothing )


urlUpdate : (pageResult -> subModel -> ( subModel, Cmd subMsg )) -> pageResult -> Model subModel -> ( Model subModel, Cmd (Msg subMsg) )
urlUpdate subUrlUpdate pageResult model =
    let
        ( newSubModel, subMsg ) =
            subUrlUpdate pageResult model.subModel
    in
        ( { model | subModel = newSubModel }, Cmd.map SubMsg subMsg )


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


scrollToBottom : String -> Cmd (Msg x)
scrollToBottom ele =
    Task.perform (always ScrolledDown) (Scroll.toBottom ele)


scrollToWindowBottom : Cmd (Msg x)
scrollToWindowBottom =
    Task.perform (always ScrolledDown) (Scroll.toWindowBottom)
