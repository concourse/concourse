module Views.CommentBar exposing
    ( Model
    , State(..)
    , Style
    , ViewState
    , defaultStyle
    , getContent
    , getTextareaID
    , handleDelivery
    , saveCallback
    , setCachedContent
    , update
    , view
    )

import Assets
import Colors
import HoverState exposing (HoverState)
import Html exposing (Attribute, Html)
import Html.Attributes exposing (id, readonly, style, value)
import Html.Events exposing (onBlur, onClick, onFocus, onInput, onMouseEnter, onMouseLeave, stopPropagationOn)
import Json.Decode as Json
import Message.Effects exposing (Effect(..), toHtmlID)
import Message.Message as Message exposing (CommentBarButtonKind(..), DomID)
import Message.Subscription exposing (Delivery(..))
import Views.Icon as Icon
import Views.Spinner as Spinner
import Views.Styles as Styles


type alias Model =
    { id : Message.DomID
    , state : State
    , style : Style
    }


type alias SaveComment =
    String -> Effect


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


{-| Provides a default set of `CommentBar.Style` settings
for cases where no special customization is required.
-}
defaultStyle : Style
defaultStyle =
    { viewBackgroundColor = "transparent"
    , editBackgroundColor = "rgba(255, 255, 255, 0.1)"
    }


{-| Retrieves the `DomID` for the given model.
-}
getTextareaID : Model -> DomID
getTextareaID model =
    Message.CommentBar model.id


{-| Retrieves the current contents of the comment bar for viewing.

    model = { state = Editing { content = "hello-world", cached = "" }, ... }
    getContent model    -- "hello-world"

-}
getContent : Model -> String
getContent model =
    case model.state of
        Viewing content ->
            content

        Editing { content } ->
            content

        Saving { content } ->
            content


{-| Updated the cached contents of the comment bar to avoid
updating the content a user may currently be editing.

    model = { state = Editing { content = "hello-world", cached = "" }, ... }

    setCachedContent "fetched-comment" model
    -- { state = Editing { content = "hello-world", cached = "fetched-comment" }, ... }

-}
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


{-| After an attempt is made to save the contents of the
comment bar, this updates its state based on whether the
operation was successful or not. For example, on a failed
save attempt, the comment bar is reset to an `Editing`
state.
-}
saveCallback : ( Bool, String ) -> Model -> Model
saveCallback ( result, content ) model =
    { model
        | state =
            case ( result, model.state ) of
                ( True, Viewing _ ) ->
                    Viewing content

                ( True, Editing state ) ->
                    Editing { state | cached = content }

                ( True, Saving _ ) ->
                    Viewing content

                ( False, Saving state ) ->
                    Editing state

                ( _, state ) ->
                    state
    }


handleDelivery : Delivery -> Model -> ( Model, List Effect )
handleDelivery delivery model =
    case delivery of
        WindowResized _ _ ->
            ( model
            , [ SyncTextareaHeight (getTextareaID model) ]
            )

        _ ->
            ( model, [] )


update : Message.Message -> SaveComment -> Model -> ( Model, List Effect )
update msg saveComment model =
    case msg of
        Message.Click (Message.CommentBarButton kind id) ->
            if model.id == id then
                case kind of
                    Message.Edit ->
                        case model.state of
                            Viewing content ->
                                ( { model | state = Editing { content = content, cached = content } }
                                , [ Focus (toHtmlID (getTextareaID model)) ]
                                )

                            _ ->
                                ( model, [] )

                    Message.Save ->
                        case model.state of
                            Editing state ->
                                if state.content /= state.cached then
                                    ( { model | state = Saving state }, [ saveComment state.content ] )

                                else
                                    ( { model | state = Viewing state.content }, [] )

                            _ ->
                                ( model, [] )

            else
                ( model, [] )

        Message.EditCommentBar id content ->
            if model.id == id then
                case model.state of
                    Editing state ->
                        ( { model | state = Editing { state | content = content } }
                        , [ SyncTextareaHeight (getTextareaID model) ]
                        )

                    _ ->
                        ( model, [] )

            else
                ( model, [] )

        _ ->
            ( model, [] )


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
         , style "cursor" "pointer"
         , if HoverState.isHovered htmlID state.hover then
            style "background-color" Colors.frame

           else
            style "background-color" "transparent"
         ]
            ++ Styles.commentBarTextButton
        )
        [ Html.text "save" ]


loadingButton : Html Message.Message
loadingButton =
    Html.div
        ([ style "background-color" "transparent"
         , style "color" Colors.buttonDisabledGrey
         ]
            ++ Styles.commentBarTextButton
        )
        [ Spinner.spinner { sizePx = 12, margin = "0" } ]


commentTextarea : Model -> Html Message.Message
commentTextarea model =
    let
        isReadOnly =
            case model.state of
                Editing _ ->
                    False

                _ ->
                    True
    in
    Html.textarea
        ([ id (toHtmlID (getTextareaID model))
         , value (getContent model)
         , onInput (Message.EditCommentBar model.id)
         , onFocus (Message.FocusCommentBar model.id)
         , onBlur (Message.BlurCommentBar model.id)
         , stopPropagationOn "keydown" (Json.succeed ( Message.NoOp, True ))
         , readonly isReadOnly
         , if isReadOnly then
            style "background-color" model.style.viewBackgroundColor

           else
            style "background-color" model.style.editBackgroundColor
         ]
            ++ Styles.commentBarTextarea
        )
        []


view : List (Attribute Message.Message) -> ViewState -> Model -> Html Message.Message
view attrs state model =
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
                    [ commentTextarea model
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
