module Views.CommentBar exposing (Effect(..), Model, State(..), Style, ViewState, commentSetCallback, defaultStyle, getContent, setCachedContent, update, updateMap, view)

import Assets
import Colors
import HoverState exposing (HoverState)
import Html exposing (Attribute, Html)
import Html.Attributes exposing (id, readonly, style, value)
import Html.Events exposing (onBlur, onClick, onFocus, onInput, onMouseEnter, onMouseLeave)
import Http
import Message.Effects exposing (Effect, toHtmlID)
import Message.Message as Message
import Views.Icon as Icon
import Views.Spinner as Spinner
import Views.Styles as Styles


type alias Model =
    { id : Message.DomID
    , state : State
    , style : Style
    }


type alias ViewState =
    { hover : HoverState
    , editable : Bool
    }


type alias Style =
    { viewBackgroundColor : String
    , editBackgroundColor : String
    }


type alias CachedState =
    { content : String
    , cached : String
    }


type State
    = Viewing String
    | Editing CachedState
    | Saving CachedState


type Effect
    = Save String


defaultStyle : Style
defaultStyle =
    { viewBackgroundColor = "transparent"
    , editBackgroundColor = "rgba(255, 255, 255, 0.1)"
    }


getContent : Model -> String
getContent model =
    case model.state of
        Viewing content ->
            content

        Editing s ->
            s.content

        Saving s ->
            s.content


setCachedContent : String -> Model -> Model
setCachedContent content model =
    { model
        | state =
            case model.state of
                Viewing _ ->
                    Viewing content

                Editing state ->
                    Editing { state | cached = content }

                Saving state ->
                    Saving { state | cached = content }
    }


commentTextArea : Model -> Html Message.Message
commentTextArea model =
    let
        isReadOnly =
            case model.state of
                Editing _ ->
                    False

                _ ->
                    True
    in
    Html.textarea
        ([ id (toHtmlID (Message.CommentBar model.id))
         , value (getContent model)
         , onInput (Message.EditCommentBar model.id)
         , onFocus (Message.FocusCommentBar model.id)
         , onBlur (Message.BlurCommentBar model.id)
         , readonly isReadOnly
         , if isReadOnly then
            style "background-color" model.style.viewBackgroundColor

           else
            style "background-color" model.style.editBackgroundColor
         ]
            ++ Styles.commentBarTextArea
        )
        []


editButton : Model -> ViewState -> Html Message.Message
editButton model state =
    let
        htmlID =
            Message.CommentBarButton Message.Edit model.id
    in
    Icon.icon
        { sizePx = 16
        , image = Assets.PencilIcon
        }
        ([ id (toHtmlID htmlID)
         , onMouseEnter (Message.Hover (Just htmlID))
         , onMouseLeave (Message.Hover Nothing)
         , onClick (Message.Click htmlID)
         , if HoverState.isHovered htmlID state.hover then
            style "background-color" Colors.sectionHeader

           else
            style "background-color" Colors.pinTools
         ]
            ++ Styles.commentBarEditButton
        )


loadingButton : Html Message.Message
loadingButton =
    Html.button
        ([ style "background-color" "transparent"
         , style "color" Colors.buttonDisabledGrey
         ]
            ++ Styles.commentBarTextButton
        )
        [ Spinner.spinner { sizePx = 12, margin = "0" } ]


saveButton : Model -> ViewState -> Html Message.Message
saveButton model state =
    Html.button
        (let
            htmlID =
                Message.CommentBarButton Message.Save model.id
         in
         [ id (toHtmlID htmlID)
         , onMouseEnter (Message.Hover (Just htmlID))
         , onMouseLeave (Message.Hover Nothing)
         , onClick (Message.Click htmlID)
         , style "color" Colors.text
         , if HoverState.isHovered htmlID state.hover then
            style "background-color" Colors.frame

           else
            style "background-color" "transparent"
         ]
            ++ Styles.commentBarTextButton
        )
        [ Html.text "save" ]


view : Model -> ViewState -> List (Attribute Message.Message) -> Html Message.Message
view model state attrs =
    Html.div
        ([ style "display" "flex"
         , style "align-items" "flex-start"
         ]
            ++ Styles.commentBarWrapper
            ++ attrs
        )
        (Icon.icon
            { sizePx = 16
            , image = Assets.MessageIcon
            }
            [ style "margin" "10px"
            , style "flex-shrink" "0"
            , style "background-size" "contain"
            , style "background-origin" "content-box"
            ]
            :: (if state.editable then
                    [ commentTextArea model
                    , Html.div
                        [ style "width" "60px"
                        , style "display" "flex"
                        , style "justify-content" "flex-end"
                        ]
                        (case model.state of
                            Viewing _ ->
                                [ editButton model state ]

                            Editing _ ->
                                [ saveButton model state ]

                            Saving _ ->
                                [ loadingButton ]
                        )
                    ]

                else
                    [ Html.pre
                        Styles.commentBarText
                        [ Html.text (getContent model) ]
                    ]
               )
        )


commentSetCallback : ( String, Result Http.Error () ) -> Model -> Model
commentSetCallback ( content, result ) model =
    let
        updatedState =
            case ( result, model.state ) of
                ( Ok (), Viewing _ ) ->
                    Viewing content

                ( Ok (), Editing state ) ->
                    Editing { state | cached = content }

                ( Ok (), Saving _ ) ->
                    Viewing content

                ( Err _, Saving state ) ->
                    Editing state

                ( _, state ) ->
                    state
    in
    { model | state = updatedState }


update : Message.Message -> Model -> ( Model, List Effect )
update msg model =
    case msg of
        Message.Click (Message.CommentBarButton kind id) ->
            if model.id == id then
                case kind of
                    Message.Edit ->
                        case model.state of
                            Viewing content ->
                                ( { model | state = Editing { content = content, cached = content } }, [] )

                            _ ->
                                ( model, [] )

                    Message.Save ->
                        case model.state of
                            Editing state ->
                                if state.content /= state.cached then
                                    ( { model | state = Saving state }, [ Save state.content ] )

                                else
                                    ( { model | state = Viewing state.content }, [] )

                            _ ->
                                ( model, [] )

            else
                ( model, [] )

        Message.EditCommentBar id content ->
            if model.id == id then
                ( { model
                    | state =
                        case model.state of
                            Editing state ->
                                Editing { state | content = content }

                            state ->
                                state
                  }
                , []
                )

            else
                ( model, [] )

        _ ->
            ( model, [] )


updateMap : Message.Message -> Model -> (Effect -> List a -> List a) -> ( Model, List a )
updateMap msg model func =
    let
        ( updatedModel, effects ) =
            update msg model
    in
    ( updatedModel
    , List.foldr
        (\effect b -> func effect b)
        []
        effects
    )
