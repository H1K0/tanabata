#include <malloc.h>
#include <string.h>
#include <time.h>

#include "../../include/core.h"

// Sappyou file signature: 七夕冊表
const uint16_t SAPPYOU_SIG[4] = {L'七', L'夕', L'冊', L'表'};

int sappyou_init(Sappyou *sappyou) {
    sappyou->created_ts = time(NULL);
    sappyou->modified_ts = sappyou->created_ts;
    sappyou->size = 0;
    sappyou->database = NULL;
    sappyou->hole_cnt = 0;
    sappyou->holes = NULL;
    sappyou->file = NULL;
    return 0;
}

int sappyou_free(Sappyou *sappyou) {
    for (uint64_t i = 0; i < sappyou->size; i++) {
        free(sappyou->database[i].name);
        free(sappyou->database[i].description);
    }
    free(sappyou->database);
    if (sappyou->file != NULL) {
        fclose(sappyou->file);
    }
    return 0;
}

int sappyou_load(Sappyou *sappyou) {
    if (sappyou->file == NULL) {
        fprintf(stderr, "Failed to load sappyou: file not specified\n");
        return 1;
    }
    uint16_t signature[4];
    rewind(sappyou->file);
    fread(signature, 2, 4, sappyou->file);
    if (memcmp(signature, SAPPYOU_SIG, 8) != 0) {
        fprintf(stderr, "Failed to load sappyou: invalid signature\n");
        return 1;
    }
    fread(&sappyou->created_ts, 8, 1, sappyou->file);
    fread(&sappyou->modified_ts, 8, 1, sappyou->file);
    fread(&sappyou->size, 8, 1, sappyou->file);
    fread(&sappyou->hole_cnt, 8, 1, sappyou->file);
    sappyou->database = malloc(sappyou->size * sizeof(Tanzaku));
    sappyou->holes = malloc(sappyou->hole_cnt * sizeof(Tanzaku *));
    size_t max_string_len = SIZE_MAX;
    for (uint64_t i = 0, r = sappyou->hole_cnt; i < sappyou->size; i++) {
        if (fgetc(sappyou->file) != 0) {
            sappyou->database[i].id = i;
            fread(&sappyou->database[i].created_ts, 8, 1, sappyou->file);
            fread(&sappyou->database[i].modified_ts, 8, 1, sappyou->file);
            getdelim(&sappyou->database[i].name, &max_string_len, 0, sappyou->file);
            getdelim(&sappyou->database[i].description, &max_string_len, 0, sappyou->file);
        } else {
            sappyou->database[i].id = HOLE_ID;
            r--;
            sappyou->holes[r] = sappyou->database + i;
        }
    }
    return 0;
}

int sappyou_save(Sappyou *sappyou) {
    if (sappyou->file == NULL) {
        fprintf(stderr, "Failed to save sappyou: file not specified\n");
        return 1;
    }
    rewind(sappyou->file);
    fwrite(SAPPYOU_SIG, 2, 4, sappyou->file);
    fwrite(&sappyou->created_ts, 8, 1, sappyou->file);
    fwrite(&sappyou->modified_ts, 8, 1, sappyou->file);
    fwrite(&sappyou->size, 8, 1, sappyou->file);
    fwrite(&sappyou->hole_cnt, 8, 1, sappyou->file);
    fflush(sappyou->file);
    for (uint64_t i = 0; i < sappyou->size; i++) {
        if (sappyou->database[i].id != HOLE_ID) {
            fputc(-1, sappyou->file);
            fwrite(&sappyou->database[i].created_ts, 8, 1, sappyou->file);
            fwrite(&sappyou->database[i].modified_ts, 8, 1, sappyou->file);
            fputs(sappyou->database[i].name, sappyou->file);
            fputc(0, sappyou->file);
            fputs(sappyou->database[i].description, sappyou->file);
            fputc(0, sappyou->file);
        } else {
            fputc(0, sappyou->file);
        }
    }
    fflush(sappyou->file);
    return 0;
}

int sappyou_open(Sappyou *sappyou, const char *path) {
    sappyou->file = fopen(path, "r+b");
    if (sappyou->file == NULL) {
        fprintf(stderr, "Failed to dump sappyou: failed to open file\n");
        return 1;
    }
    return sappyou_load(sappyou);
}

int sappyou_dump(Sappyou *sappyou, const char *path) {
    sappyou->file = fopen(path, "w+b");
    if (sappyou->file == NULL) {
        fprintf(stderr, "Failed to dump sappyou: failed to open file\n");
        return 1;
    }
    return sappyou_save(sappyou);
}

int tanzaku_add(Sappyou *sappyou, const char *name, const char *description) {
    if (sappyou->size == -1 && sappyou->hole_cnt == 0) {
        fprintf(stderr, "Failed to add tanzaku: sappyou is full\n");
        return 1;
    }
    Tanzaku newbie;
    newbie.created_ts = time(NULL);
    newbie.modified_ts = newbie.created_ts;
    size_t name_size = strlen(name),
            description_size = strlen(description);
    newbie.name = malloc(name_size + 1);
    strcpy(newbie.name, name);
    newbie.name[name_size] = 0;
    newbie.description = malloc(description_size + 1);
    strcpy(newbie.description, description);
    newbie.description[description_size] = 0;
    if (sappyou->hole_cnt > 0) {
        sappyou->hole_cnt--;
        Tanzaku **hole_ptr = sappyou->holes + sappyou->hole_cnt;
        newbie.id = *hole_ptr - sappyou->database;
        **hole_ptr = newbie;
        sappyou->holes = realloc(sappyou->holes, sappyou->hole_cnt * sizeof(Tanzaku *));
    } else {
        newbie.id = sappyou->size;
        sappyou->size++;
        sappyou->database = realloc(sappyou->database, sappyou->size * sizeof(Tanzaku));
        sappyou->database[newbie.id] = newbie;
    }
    sappyou->modified_ts = newbie.created_ts;
    return 0;
}

int tanzaku_rem(Sappyou *sappyou, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID) {
        fprintf(stderr, "Failed to remove tanzaku: got hole ID\n");
        return 1;
    }
    if (tanzaku_id >= sappyou->size) {
        fprintf(stderr, "Failed to remove tanzaku: too big ID\n");
        return 1;
    }
    if (sappyou->database[tanzaku_id].id == HOLE_ID) {
        fprintf(stderr, "Failed to remove tanzaku: target tanzaku is already removed\n");
        return 1;
    }
    sappyou->database[tanzaku_id].id = HOLE_ID;
    sappyou->hole_cnt++;
    sappyou->holes = realloc(sappyou->holes, sappyou->hole_cnt);
    sappyou->holes[sappyou->hole_cnt - 1] = sappyou->database + tanzaku_id;
    sappyou->modified_ts = time(NULL);
    return 0;
}
